package internal

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"time"

	"io"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

type Tunneller struct {
	remoteHost EndpointIface
	bastionHost EndpointIface
}

func (t *Tunneller) TunnelWithContext(ctx context.Context, cancel context.CancelFunc, localPort int) {
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "localhost", localPort))
	if err != nil {
		return
	}
	listener, ok := l.(*net.TCPListener)
	if !ok {
		log.Errorf("Could not cast listener")
		return
	}
	defer listener.Close()
	for {
		deadline := time.Now().Add(time.Second)
		listener.SetDeadline(deadline)
		select {
		case _=<-ctx.Done():
			log.Infof("Listener received shutdown signal")
			return
		default:
			// do nothing
		}
		conn, err := listener.Accept()
		if err != nil {
			if os.IsTimeout(err) {
				continue
			}
			log.Infof("Encountered unrecoverable error while attempting to accept a connection: %v", err)
			cancel()
			return
		}
		log.Debug("accepted connection")
		go t.forward(conn)
	}
}

func (t *Tunneller) forward(localConn net.Conn) {
	sshConfig, err := t.bastionHost.GetSSHConfig()
	if err != nil {
		log.Error(err)
	}

	serverConn, err := ssh.Dial("tcp", t.bastionHost.String(), sshConfig)
	if err != nil {
		log.Errorf("server dial error: %s", err)
		return
	}
	log.Debugf("connected to %s (1 of 2)", t.bastionHost.String())

	remoteConn, err := serverConn.Dial("tcp", t.remoteHost.String())
	if err != nil {
		log.Errorf("remote dial error: %s", err)
		return
	}
	log.Debugf("connected to %s (2 of 2)", t.remoteHost.String())

	copyConn := func(writer, reader net.Conn) {
		_, err := io.Copy(writer, reader)
		if err != nil {
			log.Errorf("io.Copy error: %s", err)
		}
	}
	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)
}


func Tunnel(localPort int, remoteHost EndpointIface, bastionHost EndpointIface) (chan int, error) {
	doneChannel := make(chan int)
	go func() {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", "localhost", localPort))
		if err != nil {
			return
		}
		listener, ok := l.(*net.TCPListener)
		if !ok {
			log.Errorf("Could not cast listener")
			return
		}
		defer listener.Close()
		for {
			deadline := time.Now().Add(time.Second)
			listener.SetDeadline(deadline)
			select {
			case _=<-doneChannel:
				log.Infof("Listener received shutdown signal")
				return
			default:
				// do nothing
			}
		    conn, err := listener.Accept()
			if err != nil {
				if os.IsTimeout(err) {
					continue
				}
				log.Infof("Encountered unrecoverable error while attempting to accept a connection: %v", err)
				doneChannel <- 1
				return
			}
			log.Debug("accepted connection")
			go forward(remoteHost, bastionHost, conn)
		}
	}()
	return doneChannel, nil
}

func forward(remoteHost, bastionEndpoint EndpointIface, localConn net.Conn) {
	sshConfig, err := bastionEndpoint.GetSSHConfig()
	if err != nil {
		log.Error(err)
	}

	serverConn, err := ssh.Dial("tcp", bastionEndpoint.String(), sshConfig)
	if err != nil {
		log.Errorf("server dial error: %s", err)
		return
	}
	log.Debugf("connected to %s (1 of 2)", bastionEndpoint.String())

	remoteConn, err := serverConn.Dial("tcp", remoteHost.String())
	if err != nil {
		log.Errorf("remote dial error: %s", err)
		return
	}
	log.Debugf("connected to %s (2 of 2)", remoteHost.String())

	copyConn := func(writer, reader net.Conn) {
		_, err := io.Copy(writer, reader)
		if err != nil {
			log.Errorf("io.Copy error: %s", err)
		}
	}
	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)
}
