package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

// Настройки для WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешение открытия WebSocket подключения всем клиентам
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Комната
type roomStruct struct {
	roomId   int    `json:"room_id"`
	RoomName string `json:"room_name"`
}

// Пользователь
type userStruct struct {
	userId   int    `json:"user_id"`
	userName string `json:"user_name"`
}

// Чат
type chatStruct struct {
	chatId int               `json:"chat_id"`
	room   *roomStruct       `json:"room"`
	ws     []*websocket.Conn `json:"ws"`
	user   []*userStruct     `json:"user"`
}

// Глобальные переменные

// Допустим у нас уже авторизованы 3 пользователя
var usersHub = []*userStruct{
	{userId: 1, userName: "Alex"},
	{userId: 2, userName: "Bob"},
	{userId: 3, userName: "Sam"},
}

// Допустим у нас уже созданы 3 комнаты
var roomsHub = []*roomStruct{
	{roomId: 1, RoomName: "Футбол"},
	{roomId: 2, RoomName: "Хоккей"},
	{roomId: 3, RoomName: "Баскетбол"},
}

// Допустим у нас уже созданы 3 чата (комнаты)
// Остальные поля пустые, подключения и новых пользователей мы будем добавлять при подключении к чату
var chatsHub = []*chatStruct{
	{chatId: 1, room: roomsHub[0], ws: []*websocket.Conn{}, user: []*userStruct{}},
	{chatId: 2, room: roomsHub[1], ws: []*websocket.Conn{}, user: []*userStruct{}},
	{chatId: 3, room: roomsHub[2], ws: []*websocket.Conn{}, user: []*userStruct{}},
}

//
//
//--------------------------------------------
//
//

// Создание чата
func createChat(w http.ResponseWriter, r *http.Request) {

	// Название чата
	getRoomName := r.URL.Query().Get("roomName")
	if getRoomName == "" {
		fmt.Println("Название чата пустое")
		http.NotFound(w, r)
		return
	}
	fmt.Println(getRoomName)

	fmt.Println("До")
	fmt.Printf("%+v\n", roomsHub)

	// Создаем новую комнату --------------------------------------------ID------------------------------------------
	newRoom := roomStruct{roomId: 5, RoomName: getRoomName}
	// Добавляем в массив
	roomsHub = append(roomsHub, &newRoom)

	fmt.Println("После")
	fmt.Printf("%+v\n", roomsHub)

	tmpl, err := template.ParseFiles("./start.html")
	if err != nil {
		fmt.Println("filed to show start page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Передаем на страницу массив всех чатов
	tmpl.Execute(w, roomsHub)
}

// Вывод всех чатов
func getChats(w http.ResponseWriter, r *http.Request) {

	response, err := json.Marshal(roomsHub)
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
	chatsHub[currentChatId].user = append(chatsHub[currentChatId].user, usersHub[currentUserId])
	// Добавляем подключение к конкретному чату в любом случае
	chatsHub[currentChatId].ws = append(chatsHub[currentChatId].ws, conn)

	fmt.Println("Вывод Hub:")
	fmt.Printf("%+v\n", chatsHub[currentChatId])
	fmt.Println("Присоединились к чату ID: ", chatsHub[currentChatId].room.roomId, " - ", chatsHub[currentChatId].room.RoomName)

	fmt.Printf("Количество подключений в данном чате: %v\n", len(chatsHub[currentChatId].ws))

	// Сообщение клиенту
	err = conn.WriteMessage(1, []byte(usersHub[currentUserId].userName+", добро пожаловать в чат!"))
	if err != nil {
		log.Println(err)
	}

	// В бесконечном цикле прослушиваем входящие сообщения от каждого подключенного клиента
	// Передаем ID чата, ID пользователя, ID комнаты
	reader(conn, chatsHub[currentChatId].chatId, getUserId, getRoomId)
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
			indexConn := indexOfConn(conn, chatsHub[currentChatId].ws)
			// Удаляем подключение из массива по индексу
			chatsHub[currentChatId].ws = append(chatsHub[currentChatId].ws[:indexConn], chatsHub[currentChatId].ws[indexConn+1:]...)
			fmt.Println("Удалили одно подключение из чата")

			// Вычисляем индекс удаляемого пользователя из чата
			indexUser := indexOfUser(usersHub[currentUserId], chatsHub[currentChatId].user)
			// Удаляем подключение из массива по индексу
			chatsHub[currentChatId].user = append(chatsHub[currentChatId].user[:indexUser], chatsHub[currentChatId].user[indexUser+1:]...)
			fmt.Println("Удалили одного пользователя из чата")

			// Выводим структуру
			fmt.Printf("%+v\n", chatsHub[currentChatId])
			fmt.Printf("Количество подключений в данном чате: %v\n", len(chatsHub[currentChatId].ws))
			return
		}

		log.Println("Пришло сообщение: ", string(p), " от пользователя ID ", usersHub[currentUserId].userId, " - ", usersHub[currentUserId].userName)

		// Рассылка сообщения всем участникам чата---------------------------------------------------------------------------------------------
		for i, conn := range chatsHub[currentChatId].ws {
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
		if us.userId == getUserId {
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
		if cs.chatId == chatId {
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
		if cs.room.roomId == roomId {
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
	http.HandleFunc("/1", homePage1)
	http.HandleFunc("/2", homePage2)
	http.HandleFunc("/3", homePage3)
	http.HandleFunc("/4", homePage4)
	http.HandleFunc("/5", homePage5)
	// Открываем подключение для каждого клиента по WebSocket
	http.HandleFunc("/ws", wsEndpoint)
	// Создание чата
	http.HandleFunc("/create-chat", createChat)
	// Вывод всех чатов
	http.HandleFunc("/get-chats", getChats)
}

func main() {
	fmt.Println("Сервер запущен")
	// Маршруты
	setupRoutes()
	log.Fatal(http.ListenAndServe(":8080", nil))
}
