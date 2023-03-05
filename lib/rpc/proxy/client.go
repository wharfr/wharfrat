package proxy

import (
	"fmt"
	"io"
	"net/rpc"
	"os"
	"syscall"
)

type Client struct {
	client *rpc.Client
}

func NewClient(conn io.ReadWriteCloser) *Client {
	return &Client{client: rpc.NewClient(conn)}
}

func (c *Client) Input(id uint32, fd uintptr) error {
	none := ProxyNone{}
	return c.client.Call("Proxy.Input", ProxyFDArgs{ID: id, FD: fd}, &none)
}

func (c *Client) Output(id uint32, fd uintptr) error {
	none := ProxyNone{}
	return c.client.Call("Proxy.Output", ProxyFDArgs{ID: id, FD: fd}, &none)
}

func (c *Client) IO(id uint32, fd uintptr) error {
	none := ProxyNone{}
	return c.client.Call("Proxy.IO", ProxyFDArgs{ID: id, FD: fd}, &none)
}

func (c *Client) Start() error {
	return c.client.Call("Proxy.Start", ProxyNone{}, &ProxyNone{})
}

func (c *Client) Signal(sig os.Signal) error {
	if s, ok := sig.(syscall.Signal); ok {
		return c.client.Call("Proxy.Signal", ProxySignalArgs{Signal: int(s)}, &ProxyNone{})
	}
	return fmt.Errorf("unknown Signal type: %T", sig)
}
