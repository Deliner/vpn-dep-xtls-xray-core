package traffic

import (
	"context"
	"net"
)

func startOrigin(ctx context.Context, gate *barrier) (*origin, error) {
	ctx, cancel := context.WithCancel(ctx)
	o := &origin{context: ctx, cancel: cancel, gate: gate}
	var err error
	o.tcp, err = net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, err
	}
	o.udp, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		cancel()
		o.tcp.Close()
		return nil, err
	}
	o.workers.Add(2)
	go o.serveTCP()
	go o.serveUDP()
	return o, nil
}

func (o *origin) close() {
	o.cancel()
	o.tcp.Close()
	o.udp.Close()
	o.workers.Wait()
}

func (o *origin) serveTCP() {
	defer o.workers.Done()
	for {
		connection, err := o.tcp.Accept()
		if err != nil {
			return
		}
		o.workers.Add(1)
		go o.echoTCP(connection)
	}
}

func (o *origin) echoTCP(connection net.Conn) {
	defer o.workers.Done()
	defer connection.Close()
	stop := context.AfterFunc(o.context, func() { connection.Close() })
	defer stop()
	for {
		data, err := readFrame(connection)
		if err != nil || !o.wait(data, "tcp") {
			return
		}
		if err := writeFrame(connection, data); err != nil {
			return
		}
	}
}

func (o *origin) serveUDP() {
	defer o.workers.Done()
	buffer := make([]byte, 65535)
	for {
		size, peer, err := o.udp.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		data := append([]byte(nil), buffer[:size]...)
		o.workers.Add(1)
		go o.echoUDP(data, peer)
	}
}

func (o *origin) echoUDP(data []byte, peer *net.UDPAddr) {
	defer o.workers.Done()
	if o.wait(data, "udp") {
		o.udp.WriteToUDP(data, peer)
	}
}

func (o *origin) wait(data []byte, network string) bool {
	id, sequence, err := identity(data)
	if err != nil {
		return false
	}
	ready, err := o.gate.arrive(id, sequence, network)
	if err != nil {
		return false
	}
	select {
	case <-ready:
		return true
	case <-o.context.Done():
		return false
	}
}
