package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
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

// Для первого пользователя и первого чата
func homePage1(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("./index1.html")
	if err != nil {
		fmt.Println("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// Для второго пользователя и первого чата
func homePage2(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("./index2.html")
	if err != nil {
		fmt.Println("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// Для третьего пользователя и первого чата
func homePage3(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("./index3.html")
	if err != nil {
		fmt.Println("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// Для первого пользователя и второго чата
func homePage4(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("./index4.html")
	if err != nil {
		fmt.Println("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// Для второго пользователя и второго чата
func homePage5(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("./index5.html")
	if err != nil {
		fmt.Println("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
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
			// Закрываем соединение-----------------------------------------------------------------------------------------------------
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

		// Рассылка сообщения всем участникам чата---------------------------------------------------------------------------------------------
		for i, conn := range chatsHub[currentChatId].Ws {
			if err := conn.WriteMessage(messageType, p); err != nil {
				log.Println("Ошибка при рассылке, ID подключения - ", i, " Ошибка:  ", err)
				return
			}
		}

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

	r.HandleFunc("/1", homePage1)
	r.HandleFunc("/2", homePage2)
	r.HandleFunc("/3", homePage3)
	r.HandleFunc("/4", homePage4)
	r.HandleFunc("/5", homePage5)
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

func main() {
	fmt.Println("Сервер запущен")
	// Маршруты
	setupRoutes()
	log.Fatal(http.ListenAndServe(":8080", nil))
}
