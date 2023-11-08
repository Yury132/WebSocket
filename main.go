package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// We'll need to define an Upgrader
// this will require a Read and Write buffer size
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Массив подключений
type chatStruct struct {
	ws []*websocket.Conn
}

// Глобальная переменная
var chat = chatStruct{}

// define a reader which will listen for
// new messages being sent to our WebSocket
// endpoint
func reader(conn *websocket.Conn) {
	for {
		// read in a message
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Ошибка при чтении: ", err)
			// Закрываем соединение
			err = conn.Close()
			if err != nil {
				log.Println("Ошибка при закрытии соединения: ", err)
				return
			}
			// Вычисляем индекс удаляемого подключения
			index := indexOf(conn, chat.ws)
			// Удаляем подключение из массива по индексу
			chat.ws = append(chat.ws[:index], chat.ws[index+1:]...)
			fmt.Printf("Количество подключений: %v\n", len(chat.ws))
			return
		}
		// print out that message for clarity
		log.Println("Пришло сообщение: ", string(p))

		// Проходимся по всем подключениям и всем направляем сообщение
		for i, c := range chat.ws {
			if err := c.WriteMessage(messageType, p); err != nil {
				log.Println(i, " : ", err)
				return
			}
		}

	}
}

// Вычисляем индекс элемента в срезе для Подключений
func indexOf(conn *websocket.Conn, data []*websocket.Conn) int {
	for k, v := range data {
		if conn == v {
			return k
		}
	}
	return -1
}

// Стартовая страница
func homePage(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("./index.html")
	if err != nil {
		fmt.Println("filed to show home page")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	log.Println("Client Connected")
	//log.Println("WebSocket", ws)

	// Добавляем каждое подключение в массив
	chat.ws = append(chat.ws, ws)
	fmt.Printf("Количество подключений: %v\n", len(chat.ws))

	err = ws.WriteMessage(1, []byte("Hi Client!"))
	if err != nil {
		log.Println(err)
	}

	// listen indefinitely for new messages coming
	// through on our WebSocket connection
	reader(ws)
}

func setupRoutes() {
	http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", wsEndpoint)
}

func main() {
	fmt.Println("Hello World")
	setupRoutes()
	log.Fatal(http.ListenAndServe(":8080", nil))
}
