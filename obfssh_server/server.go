package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/fangdingjun/obfssh"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"net"
)

func main() {

	var configfile string

	flag.StringVar(&configfile, "c", "config.yaml", "configure file")
	flag.Parse()

	conf, err := loadConfig(configfile)
	if err != nil {
		log.Fatal(err)
	}

	// set log level
	if conf.Debug {
		obfssh.SSHLogLevel = obfssh.DEBUG
	}

	sconf := &obfssh.Conf{
		ObfsMethod:                conf.Method,
		ObfsKey:                   conf.Key,
		DisableObfsAfterHandshake: conf.DisableObfsAfterHandshake,
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if u, err := conf.getUser(c.User()); err == nil {
				if u.Password != "" && c.User() == u.Username && string(pass) == u.Password {
					return nil, nil
				}
			}
			return nil, fmt.Errorf("password reject for user %s", c.User())
		},

		PublicKeyCallback: func(c ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			if u, err := conf.getUser(c.User()); err == nil {
				for _, pk := range u.publicKeys {
					if k.Type() == pk.Type() && bytes.Compare(k.Marshal(), pk.Marshal()) == 0 {
						return nil, nil
					}
				}
			}
			return nil, fmt.Errorf("publickey reject for user %s", c.User())
		},

		// auth log
		AuthLogCallback: func(c ssh.ConnMetadata, method string, err error) {
			if err != nil {
				obfssh.Log(obfssh.ERROR, "%s", err.Error())
				obfssh.Log(obfssh.ERROR, "%s auth failed for %s from %s", method, c.User(), c.RemoteAddr())
			} else {
				obfssh.Log(obfssh.INFO, "Accepted %s for user %s from %s", method, c.User(), c.RemoteAddr())
			}
		},
	}

	privateBytes, err := ioutil.ReadFile(conf.HostKey)
	if err != nil {
		log.Fatal(err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal(err)
	}

	config.AddHostKey(private)
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}

		obfssh.Log(obfssh.DEBUG, "accept tcp connection from %s", c.RemoteAddr())

		go func(c net.Conn) {
			sc, err := obfssh.NewServer(c, config, sconf)
			if err != nil {
				c.Close()
				obfssh.Log(obfssh.ERROR, "%s", err.Error())
				return
			}
			sc.Run()
		}(c)
	}

}
