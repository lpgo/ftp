package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

type FtpConn struct {
	userName string
	password string
	conn     net.Conn
	dataPort int
	dataIp   string
	wd       string
	pasvChan chan int
	isPasv   bool
	fileName string
}

var user string = "liupeng"
var pass string = "12345"
var base string
var commands map[string]string

func init() {
	commands = map[string]string{
		"USER": "DoUser",
		"PASS": "DoPass",
		"LIST": "DoList",
		"PORT": "DoPort",
		"RETR": "DoRetr",
		"STOR": "DoStor",
		"XPWD": "DoPwd",
		"PWD":  "DoPwd",
		"TYPE": "DoType",
		"QUIT": "DoQuit",
		"PASV": "DoPasv",
		"CWD":  "DoCwd",
		"SIZE": "DoSize",
	}

	base, _ = os.Getwd()
}

func (f *FtpConn) DoSize(strs []string) {
	file, _ := os.Open(filepath.Join(base, strs[0]))
	fileInfo, _ := file.Stat()
	log.Println(fileInfo.Name())
	f.sendString("213 " + strconv.FormatInt(fileInfo.Size(), 10))
}

func (f *FtpConn) DoCwd(strs []string) {
	dir := strs[0]
	f.wd = filepath.Join(f.wd, dir)
	f.sendString("250 change to dir.")
}

func (f *FtpConn) sendString(s string) {
	log.Printf("send: %s", s)
	f.conn.Write([]byte(s + "\r\n"))
}

func handleConn(c net.Conn, ftpConn *FtpConn) {
	ftpConn.sendString("220 Wellcom to ftp.")
	var body string
	var command string
	for {
		body = ""
		for {

			buf := make([]byte, 20)
			l, err := c.Read(buf)
			if err != nil {
				log.Fatal(err)
			}
			s := string(buf[:l])
			if strings.HasSuffix(s, "\r\n") {
				command = body + strings.Trim(s, "\r\n")
				break
			} else {
				body += s
			}
		}
		log.Println(command)
		strs := strings.Split(command, " ")

		if method, ok := commands[strings.ToUpper(strs[0])]; !ok {
			ftpConn.sendString("502 unkonw command.")
			continue
		} else {
			reflect.ValueOf(ftpConn).MethodByName(method).Call([]reflect.Value{reflect.ValueOf(strs[1:])})
			if "QUIT" == strs[0] {
				c.Close()
				break
			}
		}

	}
}

func listen(port string) (net.Listener, int) {
	l, err := net.Listen("tcp4", port)
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	p, _ := strconv.Atoi(strings.Split(l.Addr().String(), ":")[1])
	return l, p
}

func accept(l net.Listener, f *FtpConn) {
	for {
		conn, err := l.Accept()
		var ftpConn *FtpConn
		if nil == f {
			ftpConn = &FtpConn{conn: conn, wd: "/"}
		} else {
			ftpConn = f
		}

		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn, ftpConn)
	}
}

func main() {

	flag.StringVar(&user, "u", "liupeng", "set username")
	flag.StringVar(&pass, "p", "12345", "set password")
	flag.StringVar(&base, "b", base, "set base path")
	flag.Parse()

	log.SetFlags(log.Lshortfile | log.LstdFlags)
	l, _ := listen(":21")
	accept(l, nil)
	l.Close()
}

func login(f *FtpConn) bool {
	return f.userName == user && f.password == pass
}

func (f *FtpConn) DoUser(strs []string) {
	if user == strs[0] {
		f.userName = strs[0]
		f.sendString("331 user name ok,need password.")
	} else {
		f.sendString("430 user name error.")
	}

}

func (f *FtpConn) DoPass(strs []string) {
	if "" == f.userName {
		f.sendString("530 who are you")
	} else {
		f.password = strs[0]
		if login(f) {
			f.sendString("230 User logged in, proceed.")
		} else {
			f.sendString("430 pass word error.")
		}
	}

}

func (f *FtpConn) DoList(strs []string) {
	f.sendString("150 Opening data connection.")
	if f.isPasv {
		f.pasvChan <- 1
		<-f.pasvChan
	} else {
		c, _ := net.Dial("tcp", f.dataIp+":"+strconv.Itoa(f.dataPort))
		f.getFileList(c)
		defer c.Close()
	}
	f.sendString("226 Transfer complete.")
}

func (f *FtpConn) DoPort(strs []string) {
	s := strings.Split(strs[0], ",")
	h, _ := strconv.Atoi(s[4])
	l, _ := strconv.Atoi(s[5])
	f.dataPort = h*256 + l
	f.dataIp = strings.Join(s[:4], ".")
	f.sendString("200 PORT Command successful.")
}

func (f *FtpConn) DoRetr(strs []string) {
	fileName := strs[0]
	f.sendString("150 Opening data connection.")
	if f.isPasv {
		f.fileName = fileName
		f.pasvChan <- 3
		<-f.pasvChan
	} else {
		c, _ := net.Dial("tcp", f.dataIp+":"+strconv.Itoa(f.dataPort))

		fout, err := os.Open(filepath.Join(getRealPath(f.wd), fileName))
		defer fout.Close()
		defer c.Close()
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(c, fout)
	}

	f.sendString("226 Transfer complete.")
}

func (f *FtpConn) DoStor(strs []string) {
	fileName := strs[0]
	f.sendString("150 Opening data connection.")
	if f.isPasv {
		f.fileName = fileName
		f.pasvChan <- 2
		<-f.pasvChan
	} else {
		c, _ := net.Dial("tcp", f.dataIp+":"+strconv.Itoa(f.dataPort))

		fout, err := os.Create(filepath.Join(getRealPath(f.wd), fileName))
		defer fout.Close()
		defer c.Close()
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(fout, c)
	}

	f.sendString("226 Transfer complete.")

}

func (f *FtpConn) DoPwd(strs []string) {
	f.sendString(fmt.Sprintf(`257 "%s" is current directory.`, f.wd))
}

func (f *FtpConn) DoType(strs []string) {
	switch strings.ToUpper(strs[0]) {
	case "I":
		f.sendString("200 type set to I.")
	case "A":
		f.sendString("200 type set to A.")
	default:
		f.sendString("500 parm error.")
	}

}

func (f *FtpConn) DoQuit(strs []string) {
	f.sendString("221 Goodbye.")
}

func (f *FtpConn) DoPasv(strs []string) {
	l, port := listen("")
	f.pasvChan = make(chan int)
	f.isPasv = true
	go dataAccept(l, f)

	//go listen(":3763")
	strIp := strings.Replace(strings.Split(f.conn.LocalAddr().String(), ":")[0], ".", ",", -1)
	msg := fmt.Sprintf("227 Entering Passive Mode (%s,%d,%d).", strIp, port/256, port%256)
	f.sendString(msg)
}

func dataAccept(l net.Listener, f *FtpConn) {

	conn, err := l.Accept()
	if err != nil {
		log.Fatal(err)
	}

	switch <-f.pasvChan {
	case 1: //send
		f.getFileList(conn)
		f.pasvChan <- -1
	case 2: //store
		fout, err := os.Create(filepath.Join(getRealPath(f.wd), f.fileName))
		defer fout.Close()
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(fout, conn)
	case 3: //retr
		fout, err := os.Open(filepath.Join(getRealPath(f.wd), f.fileName))
		defer fout.Close()
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(conn, fout)
	}
	conn.Close()
	l.Close()
}

func (f *FtpConn) getFileList(conn net.Conn) {
	log.Println(f.wd)
	file, _ := os.Open(getRealPath(f.wd))
	defer file.Close()
	fileInfo, _ := file.Stat()
	if fileInfo.IsDir() {
		files, _ := file.Readdir(0)
		var b bytes.Buffer
		for _, fi := range files {
			b.WriteString(fi.Mode().String())
			b.WriteString("\t")

			if fi.IsDir() {
				b.WriteString("3")
			} else {
				b.WriteString("1")
			}
			b.WriteString("\t")
			b.WriteString("administrator administrator")
			b.WriteString("\t")
			b.WriteString(strconv.Itoa(int(fi.Size())))
			b.WriteString("\t")
			b.WriteString(fi.ModTime().Format("Jan 2 15:04"))
			b.WriteString("\t")
			b.WriteString(fi.Name())
			b.WriteString("\r\n")
		}
		log.Println(b.String())
		conn.Write(b.Bytes())
	} else {
		io.Copy(conn, file)
	}
}

func getRealPath(path string) string {
	return filepath.Join(base, path)
}
