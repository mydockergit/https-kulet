package server

import (
	"00pf00/https-kulet/pkg/util"
	"crypto/tls"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"net/http"
	"time"
)

type HttpServer struct {
	Cert string
	Key  string
	Addr string
}

type WS struct {
}

func (server *HttpServer) StartServer() {
	cert, err := tls.LoadX509KeyPair(server.Cert, server.Key)
	if err != nil {
		fmt.Printf("client load cert fail certpath = %s keypath = %s \n", server.Cert, server.Key)
		return
	}
	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
	//分发器
	r := mux.NewRouter()
	r.HandleFunc("/exec/{podNamespace}/{podID}/{containerName}", EXEC)
	r.HandleFunc("/cri/exec/{token}", CRI)
	s := &http.Server{
		Addr:      server.Addr,
		Handler:   r,
		TLSConfig: config,
	}
	err = s.ListenAndServeTLS("", "")
	if err != nil {
		fmt.Printf("server start fail err = %v\n", err)
	}
}
func (server *HttpServer) StartWebsocket() {
	cert, err := tls.LoadX509KeyPair(server.Cert, server.Key)
	if err != nil {
		fmt.Printf("client load cert fail certpath = %s keypath = %s \n", server.Cert, server.Key)
		return
	}
	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
	s := &http.Server{
		Addr:      server.Addr,
		Handler:   &WS{},
		TLSConfig: config,
	}
	err = s.ListenAndServeTLS("", "")
	if err != nil {
		fmt.Printf("server start fail err = %v\n", err)
	}
}

func NewHttpServer() *HttpServer {
	return &HttpServer{
		Cert: util.SERVER_CERT,
		Key:  util.SERVER_KEY,
		Addr: "0.0.0.0:10260",
	}
}

func EXEC(writer http.ResponseWriter, request *http.Request) {
	cert, err := tls.LoadX509KeyPair(util.CLIENT_CERT, util.CLIENT_KEY)
	if err != nil {
		fmt.Printf("client load cert fail certpath = %s keypath = %s \n", util.CLIENT_CERT, util.CLIENT_KEY)
		return
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: true,
		},
	}
	httpclient := &http.Client{
		Transport:     tr,
		CheckRedirect: RD,
	}
	req, err := http.NewRequest(request.Method, "https://49.51.38.39:10250"+request.URL.String(), nil)
	if err != nil {
		fmt.Printf("get request fail url = %s \n", request.URL.Host)
	}
	for k, v := range request.Header {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}
	body, err := httpclient.Do(req)
	if err != nil {
		fmt.Printf("response fail err = %v \n", err)
		return
	}
	//url := body.Request.URL.Scheme+"://"+body.Request.URL.Host+body.Header.Get("Location")
	url := body.Request.URL.Scheme + "://127.0.0.1:10250" + body.Header.Get("Location")
	http.Redirect(writer, request, url, http.StatusFound)
}

func CRI(writer http.ResponseWriter, request *http.Request) {
	upgrader := websocket.Upgrader{}
	c, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		fmt.Printf("upgrade fail err = %v\n", err)
		return
	}
	wss := request.URL
	wss.Scheme = "wss"
	wss.Host = "49.51.38.39:10250"
	cert, err := tls.LoadX509KeyPair(util.CLIENT_CERT, util.CLIENT_KEY)
	if err != nil {
		fmt.Printf("client load cert fail certpath = %s keypath = %s \n", util.CLIENT_KEY, util.CLIENT_KEY)
		return
	}
	dailer := &websocket.Dialer{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: true,
		},
	}
	wscli, _, err := dailer.Dial(wss.String(), nil)
	if err != nil {
		fmt.Printf("websocket connection failed url = %s \n", wss.Host)
		return
	}
	stop := make(chan struct{})
	//转发apiserver请求kubelet的消息
	go func(wscli, c *websocket.Conn, stop chan struct{}) {
		running := true
		for running {
			select {
			default:
				n, msg, err := c.ReadMessage()
				if err != nil {
					fmt.Printf("apiserver websocket  read fail err = %v\n", err)
					running = false
					stop <- struct{}{}
					return
				}
				err = wscli.WriteMessage(n, msg)
				if err != nil {
					fmt.Printf("kubelet websocket  write fail err = %v\n", err)
					running = false
					stop <- struct{}{}
					return
				}
			case <-stop:
				running = false
			}
		}
	}(wscli, c, stop)
	//转发kubelet响应的消息
	defer c.Close()
	running := true
	for running {
		select {
		default:
			n, msg, err := wscli.ReadMessage()
			if err != nil {
				fmt.Printf("kubelet websocket  read fail err = %v\n", err)
				running = false
				stop <- struct{}{}
			}
			err = c.WriteMessage(n, msg)
			if err != nil {
				fmt.Printf("apiserver websocket  write fail err = %v\n", err)
				running = false
				stop <- struct{}{}
			}
		}

	}
}

//服务端处理重定向
func RD(req *http.Request, via []*http.Request) error {
	//返回http.ErrUseLastResponse 禁止redirect
	return http.ErrUseLastResponse
}
func (ws *WS) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	upgrader := websocket.Upgrader{}
	c, err := upgrader.Upgrade(writer, request, nil)
	if err != nil {
		fmt.Printf("upgrade fail err = %v\n", err)
		return
	}
	//rrunning := true
	//for rrunning {
	//	mtype, msg, err := c.ReadMessage()
	//	if err != nil {
	//		fmt.Printf("server read fail err = %v\n",err)
	//		rrunning = false
	//		break
	//	}
	//	fmt.Printf("msgtype = %d msg = %s \n", mtype, string(msg))
	//}
	running := true
	for running {
		err := c.WriteMessage(websocket.TextMessage, []byte{'a'})
		if err != nil {
			fmt.Printf("websocket write fail err = %v", err)
			running = false
		}
		time.Sleep(1 * time.Second)
	}
}
