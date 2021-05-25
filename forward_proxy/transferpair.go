package forward_proxy

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// 转发连接对
type TransferPair struct {
	clientConn     net.Conn
	serverConn     net.Conn
	mu             sync.Mutex
	clientEOF      bool
	serverEOF      bool
	lastError      error
	lastActiveTime int64 // atomic
}

func NewTransferPair(clientConn net.Conn, serverConn net.Conn) (transferPair *TransferPair) {
	transferPair = &TransferPair{
		clientConn: clientConn,
		serverConn: serverConn,
	}
	transferPair.active()
	return
}

// 因为错误而关闭连接对
func (transferPair *TransferPair) closeOnError(err error) {
	transferPair.mu.Lock()
	transferPair.lastError = err
	transferPair.mu.Unlock()

	transferPair.clientConn.Close()
	transferPair.serverConn.Close()
}

// 转发活跃
func (transferPair *TransferPair) active() {
	atomic.StoreInt64(&transferPair.lastActiveTime, time.Now().Unix())
}

func (transferPair *TransferPair) client2server() {
	var buf = make([]byte, 4096)
	var size int
	var err error

	for {
		// 读client数据
		if size, err = transferPair.clientConn.Read(buf); size <= 0 {
			// 判断错误类型
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				transferPair.closeOnError(err)
			} else {
				transferPair.mu.Lock()
				transferPair.clientEOF = true
				transferPair.mu.Unlock()
				// 关闭客户端READ，服务端WRITE
				transferPair.clientConn.(*net.TCPConn).CloseRead()
				transferPair.serverConn.(*net.TCPConn).CloseWrite()
			}
			break
		}
		// 转发给server
		if _, err = transferPair.serverConn.Write(buf[:size]); err != nil {
			transferPair.closeOnError(err)
			break
		}
		transferPair.active()
	}
}

func (transferPair *TransferPair) server2client() {
	var buf = make([]byte, 4096)
	var size int
	var err error

	for {
		// 读server数据
		if size, err = transferPair.serverConn.Read(buf); size <= 0 {
			// 判断错误类型
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				transferPair.closeOnError(err)
			} else {
				transferPair.mu.Lock()
				transferPair.serverEOF = true
				transferPair.mu.Unlock()
				// 关闭服务端READ，客户端WRITE
				transferPair.serverConn.(*net.TCPConn).CloseRead()
				transferPair.clientConn.(*net.TCPConn).CloseWrite()
			}
			break
		}
		// 转发给client
		if _, err = transferPair.clientConn.Write(buf[:size]); err != nil {
			transferPair.closeOnError(err)
			break
		}
		transferPair.active()
	}
}

func (transferPair *TransferPair) DoTransfer() {
	// client -> server
	go transferPair.client2server()

	// server -> client
	go transferPair.server2client()

	// 检查转发状态
	for {
		transferPair.mu.Lock()
		lastError := transferPair.lastError
		clientEOF := transferPair.clientEOF
		serverEOF := transferPair.serverEOF
		transferPair.mu.Unlock()

		// 错误退出
		if lastError != nil {
			return
		}

		// 半关闭持续
		if (clientEOF && !serverEOF) || (!clientEOF && serverEOF) {
			if time.Now().Unix()-atomic.LoadInt64(&transferPair.lastActiveTime) >= 3 { // 半关闭持续3秒，需要杀死连接对
				transferPair.closeOnError(errors.New("半关闭状态持续太久"))
				return
			}
		} else if clientEOF && serverEOF { // 全关闭退出
			return
		}

		time.Sleep(50 * time.Millisecond)
	}
}
