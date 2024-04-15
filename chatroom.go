package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"
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
	buf = "[" + client.Name + "]" + ": " + msg
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
	message <- MakeMsg(client, "LOGIN(SYSTEM)")

	// 用户登录之后，创建一个channel，监听发送消息的动作
	isAlive := make(chan bool)

	// 用户登录之后，创建一个channel，监听退出的动作
	isQuit := make(chan bool)

	// 匿名go程读取用户输入的消息，然后广播给其他用户
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 { // 用户退出
				isQuit <- true
				return
			}
			if err != nil {
				fmt.Println("read conn err", err)
				return
			}

			// 用户输入消息转化为string类型，用MakeMsg函数封装
			msg := string(buf[:n-1])

			/*
				我们约定：输入“who”代表查询当前在线用户
				输入“rename|XXX”代表重命名为xxx
			*/
			if msg == "who" && len(msg) == 3 { // 说明是在查询在线用户列表
				conn.Write([]byte(fmt.Sprintf("%v users are online: \n", len(onlineMap))))
				//整理好信息返回给发出查询请求的用户
				for _, user := range onlineMap {
					userInfo := user.Addr + ":" + user.Name
					if user.Addr == client.Addr {
						userInfo += "(myself)" // 遍历到自己的时候加个提示，更加直观
					}
					userInfo += "\n"
					conn.Write([]byte(userInfo))
				}
			} else if strings.Split(msg, "|")[0] == "rename" { // 说明是重命名命令
				newName := strings.Split(msg, "|")[1]
				fmt.Printf("%s rename to %s", client.Name, newName)
				client.Name = newName
				onlineMap[netAddr] = client // 更新当前用户的name
				conn.Write([]byte(fmt.Sprintf("you have renamed to %s\n", newName)))
			} else {
				message <- MakeMsg(client, msg) //否则是正常发消息
			}

			isAlive <- true // 无论执行什么命令或是发送普通消息，都是活跃状态，刷新超时计时器
		}
	}()

	// 加个for循环，才能一直处理新连接，否则一个用户登录之后这个go程就关了
	for {
		select {
		case <-isQuit:
			// 用户退出需要做：
			delete(onlineMap, client.Addr)               //先将自己从在线用户map中删去
			message <- MakeMsg(client, "LOGOUT(SYSTEM)") // 广播消息：当前用户退出
			return                                       // 返回到main函数，kill掉当前用户的HandleConnect go程

		case <-isAlive: // 空case，什么也不用做，目的就是重进一下select，刷新下面的超时计时器

		// 一旦开始select就开始倒计时20秒自动踢，所以用户发消息的时候需要刷新这个计时器
		// 具体做法就是再建一个channel，用户发送消息的时候就往激活这个channel，在现在这个select里也监听这个活跃channel
		// 只要活跃channel被监听到消息，就会退出这个select，for循环重新进来，也就刷新了计时器
		case <-time.After(time.Duration(20) * time.Second):
			delete(onlineMap, client.Addr)
			message <- MakeMsg(client, "LOGOUT(SYSTEM)")
			return
		}
	}
}

// WriteMsgToClient 监听用户自己的channel是否有消息
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
