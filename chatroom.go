package main

import (
	"log"
	"net"
)

// Client 用户结构体
type Client struct {
	C    chan string
	Name string
	Addr string
}

// 全局map，存储在线用户
var onlineMap map[string]Client

// 全局channel，传递消息
var message = make(chan string)

// HandleConnect 处理客户端连接请求
func HandleConnect(conn net.Conn) {
	defer conn.Close()

	//获取用户的ip+port
	netAddr := conn.RemoteAddr().String()

	//新建用户,用户名默认是ip+port
	client := Client{
		C:    make(chan string),
		Name: netAddr,
		Addr: netAddr,
	}

	// 新连接用户添加到map中
	onlineMap[netAddr] = client

	// 创建一个专给当前用户发送消息的go程
	go WriteMsgToClient(client, conn)

	// 发送用户上线消息到全局channel
	message <- "[" + netAddr + "]" + client.Name + " login!"

	// 加个for循环，才能一直处理新连接，否则一个用户登录之后这个go程就关了
	for {

	}
}

// WriteMsgToClient 监听用户自的channel是否有消息
func WriteMsgToClient(client Client, conn net.Conn) {
	for msg := range client.C {
		conn.Write([]byte(msg + "\n"))
	}
}

func Manager() {
	// 初始化全局onlineMap空间
	onlineMap = make(map[string]Client)

	for {
		// 监听全局channel到来的数据
		msg := <-message

		//循环发送消息给所有在线用户
		for _, client := range onlineMap {
			client.C <- msg //消息写入到每个用户的channel中
		}
	}
}

func main() {

	// 创建监听socket的主go程
	listener, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		log.Fatal("listen err:", err.Error())
		return
	}
	defer listener.Close()

	// 全局管理go程，管理全局map和channel
	go Manager()

	// 循环监听客户端连接请求
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("accept err:", err.Error())
			return
		}

		// 处理客户端数据请求
		go HandleConnect(conn)
	}
}
