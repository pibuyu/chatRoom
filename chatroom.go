package main

import (
	"fmt"
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

func MakeMsg(client Client, msg string) (buf string) {
	buf = "[" + client.Addr + "]" + client.Name + " :" + msg
	return
}

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

	// 创建一个go程，专用于：发送消息给当前用户。把连接和client传过去，conn往client.C写入数据
	go WriteMsgToClient(client, conn)

	//// 发送用户上线消息到全局channel
	//message <- "[" + netAddr + "]" + client.Name + " login!"
	message <- MakeMsg(client, "login") // 不再写死消息，封装成函数

	// 匿名go程读取用户输入的消息，然后广播给其他用户
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				fmt.Printf("user : %s exit", client.Name)
				return
			}
			if err != nil {
				fmt.Println("read conn err", err)
				return
			}

			// 用户输入消息转化为string类型，用MakeMsg函数封装
			msg := string(buf[:n-1])

			if msg == "who" && len(msg) == 3 { // 说明是在查询在线用户列表
				conn.Write([]byte("online user list: \n"))
				//整理好信息返回给这个用户
				for _, user := range onlineMap {
					userInfo := user.Addr + ":" + user.Name
					if user.Addr == client.Addr {
						userInfo += "(myself)" // 遍历到自己的时候加个提示，更加直观
					}
					userInfo += "\n"
					conn.Write([]byte(userInfo))
				}
			} else {
				message <- MakeMsg(client, msg) //否则是正常发消息
			}

		}
	}()

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

		// 全局channel的每一条消息都循环发送消息给所有在线用户
		for _, client := range onlineMap {
			// 消息发送到每个用户的C中，激活了WriteMsgToClient函数，遍历C中的消息，conn往客户端写
			// 即给用户发送消息使用的是WriteMsgToClient函数
			client.C <- msg
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
