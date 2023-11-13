package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Настройки для WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешение открытия WebSocket подключения всем клиентам
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Вспомогательная структура - ID пользователя и комнаты для перехода в конкретный чат
type userAndRoomStruct struct {
	UserId int `json:"user_id"`
	RoomId int `json:"room_id"`
}

// Комната
type roomStruct struct {
	RoomId   int    `json:"room_id"`
	RoomName string `json:"room_name"`
}

// Пользователь
type userStruct struct {
	UserId   int    `json:"user_id"`
	UserName string `json:"user_name"`
}

// Чат
type chatStruct struct {
	ChatId int               `json:"chat_id"`
	Room   *roomStruct       `json:"room"`
	Ws     []*websocket.Conn `json:"ws"`
	User   []*userStruct     `json:"user"`
}

// Передаваемое сообщение в Nats
type SendMessage struct {
	Msg         string `json:"msg"`
	MessageType int    `json:"messageType"`
	ChatId      int    `json:"chatId"`
}

// Глобальные переменные

// Hub всех пользователей
var usersHub = []*userStruct{}

// Hub всех комнат
var roomsHub = []*roomStruct{}

// Hub всех чатов
var chatsHub = []*chatStruct{}

var store = sessions.NewCookieStore([]byte("super-secret-key"))

// Уникальный ID автоинкремент для чатов и комнат
var globalId = 1

// Поток Nats
var js jetstream.JetStream

// var ctx context.Context
// var cancel context.CancelFunc
var stream jetstream.Stream

//
//
//-----------------------------------------------------------------------------------------------------------
//
//

// Для разных пользователей нужно открывать разные браузеры
// Страница после прохождения авторизации
func start(w http.ResponseWriter, r *http.Request) {

	// ID пользователя
	userId, err := strconv.Atoi(r.URL.Query().Get("userId"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println("failed to get id from string")
		return
	}

	// Костыль и проверка на то, что данного пользователя не было в массиве - поиск по ID
	if userId != -1 && indexU(userId) == -1 {
		// Создаем нового пользователя
		newUser := userStruct{UserId: userId, UserName: r.URL.Query().Get("userName")}
		// Добавляем в массив
		usersHub = append(usersHub, &newUser)

		// // Задаем жизнь сессии в секундах
		// // 10 мин
		// store.Options = &sessions.Options{
		// 	MaxAge: 60 * 10,
		// }
		//
		// Создаем сессию
		session, err := store.Get(r, "session-name")
		if err != nil {
			fmt.Println("session create failed")
		}

		// Сохраняем данные пользователя
		// Будем получать userId и userName из гугл авторизации!!!!!!!!!!!!!!!!!!!!!!!!!!!Следить чтобы не пересохранялись пустые значения......
		session.Values["userId"] = r.URL.Query().Get("userId")
		session.Values["userName"] = r.URL.Query().Get("userName")
		if err = session.Save(r, w); err != nil {
			fmt.Println("filed to save session")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Получаем сессию
		// session, err = store.Get(r, "session-name")
		// if err != nil {
		// 	fmt.Println("session failed")
		// 	w.WriteHeader(http.StatusInternalServerError)
		// 	return
		// }

		// Читаем данные из сессии
		// usid := session.Values["userId"].(string)

		// fmt.Println("ID пользователя из Сессии:")
		// fmt.Println(usid)
	}

	tmpl, err := template.ParseFiles("./start.html")
	if err != nil {
		fmt.Println("filed to show start page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Передаем на страницу массив всех чатов
	tmpl.Execute(w, roomsHub)
}

// Создание чата
func createChat(w http.ResponseWriter, r *http.Request) {

	// Название чата из формы POST запрос
	getRoomName := r.FormValue("chatName")
	if getRoomName == "" {
		fmt.Println("Название чата пустое")
		// Переадресуем пользователя на ту же страницу
		// Костыль userId == -1
		http.Redirect(w, r, "/start?userId=-1&userName=xxx", http.StatusSeeOther)
		return
	}
	fmt.Println(getRoomName)

	// Создаем новую комнату
	newRoom := roomStruct{RoomId: globalId, RoomName: getRoomName}
	// Добавляем в массив
	roomsHub = append(roomsHub, &newRoom)

	// Создаем новый чат
	newChat := chatStruct{ChatId: globalId, Room: &newRoom, Ws: []*websocket.Conn{}, User: []*userStruct{}}
	// Добавляем в массив
	chatsHub = append(chatsHub, &newChat)

	globalId++
	fmt.Println("globalId ", globalId)

	// Переадресуем пользователя на ту же страницу
	// Костыль userId == -1
	http.Redirect(w, r, "/start?userId=-1&userName=xxx", http.StatusSeeOther)
}

// Переходим в конкретный чат
func goChat(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	// ID чата
	chatId, err := strconv.Atoi(vars["chatId"])
	if err != nil || chatId < 1 {
		http.NotFound(w, r)
		return
	}

	// Получаем сессию
	session, err := store.Get(r, "session-name")
	if err != nil {
		fmt.Println("session failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Из сессии читаем ID пользователя
	userIdString := session.Values["userId"].(string)
	// Переводим в int
	userIdInt, err := strconv.Atoi(userIdString)
	if err != nil || userIdInt < 1 {
		http.NotFound(w, r)
		return
	}

	fmt.Println("ID пользователя из Сессии: ", userIdInt)

	// Формируем структуру
	data := userAndRoomStruct{UserId: userIdInt, RoomId: chatId}

	tmpl, err := template.ParseFiles("./index1.html")
	if err != nil {
		fmt.Println("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Передаем данные
	tmpl.Execute(w, data)
}

// Вывод всех чатов
func getChats(w http.ResponseWriter, r *http.Request) {
	response, err := json.Marshal(chatsHub)
	if err != nil {
		fmt.Println("filed to marshal response data")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(response)
}

// Вывод всех комнат
func getRooms(w http.ResponseWriter, r *http.Request) {
	response, err := json.Marshal(roomsHub)
	if err != nil {
		fmt.Println("filed to marshal response data")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(response)
}

// Вывод всех пользователей
func getUsers(w http.ResponseWriter, r *http.Request) {
	response, err := json.Marshal(usersHub)
	if err != nil {
		fmt.Println("filed to marshal response data")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(response)
}

// Подключение через WebSocket
// Сюда приходят все клиенты
func wsEndpoint(w http.ResponseWriter, r *http.Request) {

	// ID пользователя
	getUserId, err := strconv.Atoi(r.URL.Query().Get("userId"))
	if err != nil || getUserId < 1 {
		http.NotFound(w, r)
		return
	}
	// ID комнаты
	getRoomId, err := strconv.Atoi(r.URL.Query().Get("roomId"))
	if err != nil || getRoomId < 1 {
		http.NotFound(w, r)
		return
	}

	// Уникальное подключение *websocket.Conn
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	log.Println("Подключился новый пользователь, userId - ", getUserId, "roomId - ", getRoomId)

	// -------------------------------------------------------------------------------------------------Сохранение *websocket.Conn-----------------------------

	// Ищем текущий индекс пользователя
	currentUserId := indexU(getUserId)

	// Ищем индекс чата, к которому присоединился пользователь
	currentChatId := indexChRoom(getRoomId)

	// Добавляем пользователя к конкретному чату
	chatsHub[currentChatId].User = append(chatsHub[currentChatId].User, usersHub[currentUserId])
	// Добавляем подключение к конкретному чату в любом случае
	chatsHub[currentChatId].Ws = append(chatsHub[currentChatId].Ws, conn)

	fmt.Println("Вывод Hub:")
	fmt.Printf("%+v\n", chatsHub[currentChatId])
	fmt.Println("Присоединились к чату ID: ", chatsHub[currentChatId].Room.RoomId, " - ", chatsHub[currentChatId].Room.RoomName)

	fmt.Printf("Количество подключений в данном чате: %v\n", len(chatsHub[currentChatId].Ws))

	// Сообщение клиенту
	err = conn.WriteMessage(1, []byte(usersHub[currentUserId].UserName+", добро пожаловать в чат!"))
	if err != nil {
		log.Println(err)
	}

	// В бесконечном цикле прослушиваем входящие сообщения от каждого подключенного клиента
	// Передаем ID чата, ID пользователя, ID комнаты
	reader(conn, chatsHub[currentChatId].ChatId, getUserId, getRoomId)
}

// В бесконечном цикле прослушиваем входящие сообщения от каждого подключенного клиента
// Передаем ID чата, ID комнаты, ID пользователя - Уникальные данные для каждого клиента, чьи сообщения мы прослушиваем
// Получаем соответствие *websocket.Conn со всеми ID
func reader(conn *websocket.Conn, chatId int, userId int, roomId int) {
	// Этот бесконечный цикл запускаетя для каждого клиента с открытым WebSocket подключением
	for {
		// Ждем сообщение от клиента
		messageType, p, err := conn.ReadMessage()

		// Ищем текущий индекс пользователя
		currentUserId := indexU(userId)

		// Ищем текущий индекс чата
		currentChatId := indexCh(chatId)

		// При ошибке закрываем соединение - удаляем подключение и клиента из массива
		if err != nil {
			log.Println("Ошибка при чтении: ", err)
			// Закрываем соединение
			err = conn.Close()
			if err != nil {
				log.Println("Ошибка при закрытии соединения: ", err)
				return
			}
			// Вычисляем индекс удаляемого подключения из чата
			indexConn := indexOfConn(conn, chatsHub[currentChatId].Ws)
			// Удаляем подключение из массива по индексу
			chatsHub[currentChatId].Ws = append(chatsHub[currentChatId].Ws[:indexConn], chatsHub[currentChatId].Ws[indexConn+1:]...)
			fmt.Println("Удалили одно подключение из чата")

			// Вычисляем индекс удаляемого пользователя из чата
			indexUser := indexOfUser(usersHub[currentUserId], chatsHub[currentChatId].User)
			// Удаляем подключение из массива по индексу
			chatsHub[currentChatId].User = append(chatsHub[currentChatId].User[:indexUser], chatsHub[currentChatId].User[indexUser+1:]...)
			fmt.Println("Удалили одного пользователя из чата")

			// Выводим структуру
			fmt.Printf("%+v\n", chatsHub[currentChatId])
			fmt.Printf("Количество подключений в данном чате: %v\n", len(chatsHub[currentChatId].Ws))
			return
		}

		log.Println("Пришло сообщение: ", string(p), " от пользователя ID ", usersHub[currentUserId].UserId, " - ", usersHub[currentUserId].UserName)

		// Готовим сообщение для отправки
		msg := SendMessage{
			Msg:         string(p),
			MessageType: messageType,
			ChatId:      currentChatId,
		}

		// Кодируем
		b, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("js message marshal err")
			return
		}

		// Отправляем полученное сообщение в Nats
		if _, err = js.Publish(context.Background(), "events.us.page_loaded", b); err != nil {
			fmt.Println("failed to publish message", err)
			return
		}

		// // Рассылка сообщения всем участникам чата---------------------------------------------------------------------------------------Рассылка----------------------------
		// for i, conn := range chatsHub[currentChatId].Ws {
		// 	if err := conn.WriteMessage(messageType, p); err != nil {
		// 		log.Println("Ошибка при рассылке, ID подключения - ", i, " Ошибка:  ", err)
		// 		return
		// 	}
		// }

	}
}

// Вычисляем индекс элемента в срезе для последующего удаления - Подключения
func indexOfConn(conn *websocket.Conn, data []*websocket.Conn) int {
	for k, v := range data {
		if conn == v {
			return k
		}
	}
	return -1
}

// Вычисляем индекс элемента в срезе для последующего удаления - Пользователи
func indexOfUser(user *userStruct, data []*userStruct) int {
	for k, v := range data {
		if user == v {
			return k
		}
	}
	return -1
}

// Ищем индекс пользователя в usersHub
func indexU(getUserId int) int {
	// Ищем индекс пользователя
	currentUserId := -1
	for i, us := range usersHub {
		if us.UserId == getUserId {
			currentUserId = i
			break
		}
	}
	if currentUserId == -1 {
		fmt.Println("Не нашли пользователя!!! Ошибка...")
		currentUserId = -1
	}
	return currentUserId
}

// Ищем индекс чата в chatsHub
func indexCh(chatId int) int {
	// Ищем текущий индекс чата
	currentChatId := -1
	for i, cs := range chatsHub {
		if cs.ChatId == chatId {
			currentChatId = i
			break
		}
	}
	if currentChatId == -1 {
		fmt.Println("Не нашли чат!!! Ошибка...")
		currentChatId = -1
	}
	return currentChatId
}

// Ищем индекс чата в chatsHub по ID комнаты
func indexChRoom(roomId int) int {
	currentChatId := -1
	for i, cs := range chatsHub {
		if cs.Room.RoomId == roomId {
			currentChatId = i
			break
		}
	}
	if currentChatId == -1 {
		fmt.Println("Не нашли чат по комнате!!! Ошибка...")
		currentChatId = -1
	}
	return currentChatId
}

// Маршруты
func setupRoutes() {
	r := mux.NewRouter()
	// Открываем подключение для каждого клиента по WebSocket
	r.HandleFunc("/ws", wsEndpoint)
	// Создание чата
	r.HandleFunc("/create-chat", createChat)
	// Вывод всех комнат
	r.HandleFunc("/get-rooms", getRooms)
	// Вывод всех чатов
	r.HandleFunc("/get-chats", getChats)
	// Вывод всех пользователей
	r.HandleFunc("/get-users", getUsers)
	// Страница после прохождения авторизации
	r.HandleFunc("/start", start)
	// Переход в конкретный чат
	r.HandleFunc("/go-chat/{chatId:[0-9]+}", goChat)
	http.Handle("/", r)
}

// // Отображаем текущее состояние потока
// func printStreamState(ctx context.Context, stream jetstream.Stream) {
// 	info, _ := stream.Info(ctx)
// 	b, _ := json.MarshalIndent(info.State, "", " ")
// 	fmt.Println(string(b))
// }

// Воркер
func worker(id int, jobs <-chan *SendMessage) {
	// Ожидаем получения данных для работы
	// Если данных нет в канале - блокировка
	for j := range jobs {
		//fmt.Println("worker", id, "принял сообщение: ", j)
		// Рассылка сообщения всем участникам чата
		for i, conn := range chatsHub[j.ChatId].Ws {
			if err := conn.WriteMessage(j.MessageType, []byte(j.Msg)); err != nil {
				log.Println("Ошибка при рассылке, ID подключения - ", i, " Ошибка:  ", err)
				continue
			}
		}
		//fmt.Println("worker", id, "разослал сообщение: ", j)
	}
}

func main() {

	// Канал для воркеров
	jobs := make(chan *SendMessage, 100)

	// Сразу запускаем воркеров в горутинах
	// Они будут ожидать получения данных для работы
	for w := 1; w <= 3; w++ {
		go worker(w, jobs)
	}

	// Адрес сервера nats
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = nats.DefaultURL
	}

	// Подключаемся к серверу
	nc, err := nats.Connect(url)
	if err != nil {
		fmt.Println("Ошибка при Connect...")
	}
	defer nc.Drain()

	js, err = jetstream.New(nc)
	if err != nil {
		fmt.Println("Ошибка при jetstream.New...")
	}

	cfg := jetstream.StreamConfig{
		Name: "EVENTS",
		// Очередь
		Retention: jetstream.WorkQueuePolicy,
		Subjects:  []string{"events.>"},
	}

	// При таймауте прога ломается после истечения времени!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
	// ctx, cancel = context.WithTimeout(context.Background(), 50*time.Second)
	// defer cancel()

	// Создаем поток
	stream, err = js.CreateStream(context.Background(), cfg)
	if err != nil {
		fmt.Println("Ошибка при js.CreateStream...")
	}

	// Создаем получателя
	cons, err := stream.CreateOrUpdateConsumer(context.Background(), jetstream.ConsumerConfig{
		Name: "processor-1",
	})
	if err != nil {
		fmt.Println("Ошибка при stream.CreateOrUpdateConsumer...")
	}

	// В горутине получатель беспрерывно ждет входящих сообщений
	// При получении сообщений, передает задачи-данные-смс воркерам для последующей рассылки
	go func() {
		_, err := cons.Consume(func(msg jetstream.Msg) {

			// Декодируем
			var info = new(SendMessage)
			if err := json.Unmarshal(msg.Data(), info); err != nil {
				fmt.Println("Ошибка при декодировании....")
			} else {
				fmt.Println("Полученные данные Consume: ", info)
				//fmt.Println("Отправляем данные в канал")
				// Заполняем канал данными
				// Воркеры начнут работать
				jobs <- info
			}
			// Подтверждаем получение сообщения
			err := msg.DoubleAck(context.Background())
			if err != nil {
				fmt.Println("Ошибка при DoubleAck...")
			}
		})
		if err != nil {
			fmt.Println("Ошибка при Consume...")
		}
	}()

	// Маршруты
	setupRoutes()

	// Фиксируем нажатие Ctrl+C для остановки программы
	shutdown := make(chan os.Signal, 1)
	// Оповещаем канал
	signal.Notify(shutdown, syscall.SIGINT)

	// Запускаем сервер
	go func() {
		fmt.Println("Сервер запущен")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			fmt.Println("failed to start server")
		}
	}()

	// Ждем нажатия Ctrl+C
	<-shutdown

}
