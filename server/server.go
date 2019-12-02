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

func ReadAddr(r *bufio.Reader) (string, error) {
	version, _ := r.ReadByte()
	log.Printf("client protocol version: %d", version)
	if version != 5 {
		return "", errors.New("this is not socks5 protocol")
	}
	cmd, _ := r.ReadByte()

	log.Printf("客户端请求的类型是: %d", cmd)
	if cmd != 1 {
		return "", errors.New("客户端请求类型不为\"1\",即请求类型必须是代理连接")
	}

	r.ReadByte() // 跳过RSV 字段,即RSV 保留字段, 值长度为1个字节

	addrtype, _ := r.ReadByte()
	log.Printf("客户端请求的远程服务器地址类型是: %d", addrtype)

	if addrtype != 3 {
		return "", errors.New("请求的远程服务器地址类型部为\"3\",即请求的远程服务器地址必须是域名")
	}

	addrlen, _ := r.ReadByte()
	addr := make([]byte, addrlen)
	io.ReadFull(r, addr)
	log.Printf("域名为: %s", addr)

	var port int16
	binary.Read(r, binary.BigEndian, &port)

	return fmt.Sprintf("%s:%d", addr, port), nil
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)

	addr, err := ReadAddr(r)
	if err != nil {
		log.Print(err)
	}
	log.Print("得到的完整的地址是: ", addr)

	resp := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	conn.Write(resp)

	var (
		remote net.Conn
	)

	remote, err = net.Dial("tcp", addr)
	if err != nil {
		log.Print(err)
		conn.Close()
		return
	}

	wg := new(sync.WaitGroup)
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(remote, r)
		remote.Close()
	}()

	go func() {
		defer wg.Done()
		io.Copy(conn, remote)
		conn.Close()
	}()

	wg.Wait()
}

func main() {

	fmt.Println("Lanuching server...")

	ln, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatal(err)
	}

	/*
			for {
				message, _ := bufio.NewReader(conn).ReadString('\n')

				fmt.Print("Message Received:", string(message))
				newmessage := strings.ToUpper(message)
				conn.Write([]byte(newmessage + "\n"))
		    }
	*/

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go handleConn(conn)
	}
}
