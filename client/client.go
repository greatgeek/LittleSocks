package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

func HandShake(r *bufio.Reader, conn net.Conn) error {
	version, _ := r.ReadByte() // 版本号
	log.Printf("version: %d", version)

	if version != 5 {
		return errors.New("this protocol is not socks5")
	}

	nmethods, _ := r.ReadByte() // nmethods 记录methods的长度
	log.Printf("length of methods: %d", nmethods)

	buf := make([]byte, nmethods)
	io.ReadFull(r, buf)
	log.Printf("authentication: %v", buf)

	resp := []byte{5, 0}
	conn.Write(resp)
	return nil
}

func ReadAddr(r *bufio.Reader) (string, error) {
	version, _ := r.ReadByte()
	log.Printf("client protocol version: %d", version)

	if version != 5 {
		return "", errors.New("this is not socks5 protocol")
	}

	cmd, _ := r.ReadByte()

	log.Printf("客户端请求的类型是: %d", cmd)
	if cmd != 1 {
		return "", errors.New("客户端请求类型不为“1”，即请求必须是代理连接！")
	}

	r.ReadByte() // 跳过 RSV字段， 即RSV保留字段， 值长度为1个字节

	addrtype, _ := r.ReadByte()
	log.Printf("客户端请求的远程服务器地址类型是：%d", addrtype) /* "addrtype"代表请求的远程服务器地址类型，它是一个可变参数，但它的值长度为1个字节，
	有三种类型：
							1. 数字“1”：表示是一个IPv4地址（IP v4 address）；
							2. 数字“2”：表示是一个域名（DOMAINNAME）；
							3. 数字“3”：表示是一个IPv6地址（IP v6 address）；

	*/
	if addrtype != 3 { //表示只处理请求的远程服务器地址类型是域名
		return "", errors.New("请求的远程服务器地址类型部位“3”，即请求的远程服务器地址必须是域名！")
	}

	addrlen, _ := r.ReadByte()    //读取一个字节以得到域名的长度，因为服务器地址类型长度就是“1”，所以它无论是IP还是域名我们都能获取到完整的内容。如果能走到这一步代码说明一定是域名，如果没有上面的一行过滤代码我们就还需要考滤IPV4和IPV6两种情况了
	addr := make([]byte, addrlen) //定义一个和域名长度一样大小的容器
	io.ReadFull(r, addr)          // 将域名的内容读取出来
	log.Printf("域名为：%s", addr)

	var port int16                          // 因为端口需要用2个字节来表示，所以我们用int16来定义它的取值范围
	binary.Read(r, binary.BigEndian, &port) // 读取2个字节，并将读取到的内容赋值给port变量

	return fmt.Sprintf("%s:%d", addr, port), nil

}

func HandleConn(localConn net.Conn, remoteConn net.Conn) {
	defer localConn.Close()
	r := bufio.NewReader(localConn) //装饰模式
	HandShake(r, localConn)

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(remoteConn, r)
	}()

	go func() {
		defer wg.Done()
		io.Copy(localConn, remoteConn)
		localConn.Close()
	}()

	wg.Wait()
}

func main() {
	listener, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatal(err)
	}

	for {
		/*

			// read in input from stdin
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Text to send: ")
			text, _ := reader.ReadString('\n')

			// send to socket
			fmt.Fprintf(conn, text+"\n")
			// listen for reply
			message, _ := bufio.NewReader(conn).ReadString('\n')
			fmt.Print("Message from server: " + message)

		*/

		localConn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		remoteConn, err := net.Dial("tcp", "10.170.44.218:8081")
		if err != nil {
			log.Fatal(err)
		}

		go HandleConn(localConn, remoteConn)
	}
}
