package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

/*
1. know what is socks5 protocol
2. handshake
3. get the request from client proxy
4. start proxy
*/

func HandShake(r *bufio.Reader, conn net.Conn) error {
	version, _ := r.ReadByte() // 用"bufio.Reader" 的"ReadByte"方法读取一个字节，即 socks的版本号
	log.Printf("version: %d", version)

	if version != 5 {
		return errors.New("this protocol is not socks5")
	}
	nmethods, _ := r.ReadByte() // nmethods 是记录methods的长度的， nmethods的长度是1个字节。 methods表示客户端支持的验证方式， 可以有多种，1-255个字节
	log.Printf("length of methods: %d", nmethods)

	buf := make([]byte, nmethods)
	io.ReadFull(r, buf)                   // 这个方法和“io.Copy”效果看起来相反， “io.ReadFull” 循环读取“r”的数据并依次写入到“buf”中，直到把“buf”写满为止
	log.Printf("authentication: %v", buf) /*	常见的几种方式如下：
		1. 数字“0”：表示不需要用户名或者密码验证；
		2. 数字“1”：GSSAPI是SSH支持的一种验证方式；
		3. 数字“2”：表示需要用户名和密码进行验证；
		4. 数字“3”至“7F”：表示用于IANA分配（IANA ASSIGNED）
		5. 数字“80”至“FE”：表示私人方法保留（RESERVED FOR PRIVATE METHODS）
		6. 数字“FF”：不支持所有的验证方式，这样的话就无法进行连接了
	*/

	resp := []byte{5, 0} // 以上操作实现了接受客户端消息，所以服务器需要回应客户端消息。第一个参数表示版本号为5，即socks5协议，第二个参数表示认证方式为0， 即无需密码访问
	conn.Write(resp)
	return nil
}

func ReadAddr(r *bufio.Reader) (string, error) {
	version, _ := r.ReadByte()
	log.Printf("client protocol version: %d", version)
	if version != 5 {
		return "", errors.New("this is not socks5 protocol")
	}
	cmd, _ := r.ReadByte() /* 从上一次读取的位置再往下读取一个字节， cmd代表客户端请求的类型，值长度也是一个字节，
	有三种类型：
	1. 数字“1”：表示客户端需要你帮忙代理连接，即CONNECT;
	2. 数字“2”：表示让你代理服务器，帮他建立端口，即BIND;
	3. 数字“3”：表示UDP连接请求用来建立一个在UDP延迟过程中操作UDP数据报的连接，即UDP ASSOCIATE；
	*/

	log.Printf("客户端请求的类型是：%d", cmd)
	if cmd != 1 { //此处表示我们只处理客户端请求类型为“1”的连接
		return "", errors.New("客户端请求类型不为“1”,即请求类型必须是代理连接!.")
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

func handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn) //把“conn”进行包装，这样方便我们处理“conn”的数据
	HandShake(r, conn)         //进行握手，该函数是建立服务端和客户端的连接，但是仅仅建立握手就没有用，只是服务器收到了客户端的请求，我们还要继续往下走
	addr, err := ReadAddr(r)   // 获取客户端代码的请求，即让客户端发起请求，告诉Socks服务端客户端需要访问哪个远程服务器，其中包含，远程服务器的地址和端口，地址可以是IPv4，IPv6，也可以是域名。
	if err != nil {
		log.Print(err)
	}
	log.Print("得到的完整的地址是： ", addr) //注意：HTTP对应的端口是80端口，HTTPS对应的端口是443端口
	resp := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	conn.Write(resp) //现在客户端把要请求的远程服务器的信息都告诉SOCKS5服务器了，那么SOCKET5代理服务器就可以和远程服务器建立连接了，不管连接是否成功，都要给客户端回应。

	// 实现代理部分需要字节填充。首先你得会用switchomega软件来调试上面的代码
	var (
		remote net.Conn // 定义远端的服务器连接
	)

	remote, err = net.Dial("tcp", addr) //建立到目标服务器的连接
	if err != nil {
		log.Print(err)
		conn.Close()
		return
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(remote, r) //读取原地址请求（conn）,然后将读取到的数据发送给目标主机。这里建议用“r”，不建议用conn，因为它有重传机制
		remote.Close()
	}()

	go func() {
		defer conn.Close()
		io.Copy(conn, remote) //与上面相反，就是将目标主机的数据返回给客户端
		conn.Close()
	}()

	wg.Wait()
}

func main() {
	flag.Parse()
	listener, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}
