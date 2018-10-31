package session

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type processEntry func(tokens []string) bool

// AuthenticatorFunc is the function that will be called
// by the FTP Server as soon as the authentcation process completes
// (ie USER+PASS). If you return true the user is considered
// authenticated from there on
type AuthenticatorFunc func(name, password string) bool

func (ses *Session) processSYST(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "SYST"}).Info("session::Session::processSYST method begin")
	ses.sendStatement("215 UNIX Type: L8")
	return false
}

func (ses *Session) processQUIT(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "QUIT"}).Info("session::Session::processQUIT method begin")
	ses.sendStatement("221 Goodbye.")
	return true
}

func (ses *Session) processNOOP(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "NOOP"}).Info("session::Session::processNOOP method begin")
	ses.sendStatement("200 NOOP ok.")
	return false
}
func (ses *Session) processFEAT(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "FEAT"}).Info("session::Session::processFEAT method begin")
	buf := new(bytes.Buffer)

	buf.WriteString("211-Features:\r\n")

	for _, cmd := range commands {
		buf.WriteString(fmt.Sprintf(" %s\r\n", cmd))
	}

	if ses.cert != nil && !ses.conn.IsSecure() {
		buf.WriteString(fmt.Sprintf(" %s\r\n", "AUTH"))
	}

	buf.WriteString("211 End")

	ses.sendStatement(buf.String())

	return false
}

func (ses *Session) processPWD(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "PWD"}).Info("session::Session::processPWD method begin")
	ses.sendStatement(fmt.Sprintf("257 \"%s\"", ses.fileProvider.CurrentDirectory()))
	return false
}

func (ses *Session) processCDUP(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "CDUP"}).Info("session::Session::processCDUP method begin")
	return ses.processCWD([]string{"CWD", ".."})
}

func (ses *Session) processCWD(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "CWD"}).Info("session::Session::processCWD method begin")

	if len(tokens) < 2 {
		ses.sendStatement("550 Failed to change directory")
		return false
	}

	path := strings.Join(tokens[1:], " ")
	path = clearPath(path)

	err := ses.fileProvider.ChangeDirectory(path)
	if err != nil {
		ses.sendStatement("550 Failed to change directory")
		return false
	}

	ses.sendStatement("250 Directory successfully changed")

	return false
}

func (ses *Session) processRETR(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "RETR"}).Info("session::Session::processRETR method begin")

	rest := ses.lastREST
	ses.lastREST = 0

	if len(tokens) < 2 {
		ses.sendStatement("501 object needed!")
		return false
	}

	file := clearPath(strings.Join(tokens[1:], " "))

	f, err := ses.fileProvider.Get(file)
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "RETR", "file": file, "f": f, "err": err}).Debug("session::Session::processRETR method after ses.fileProvider.Get(file)")

	if err != nil {
		log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processRETR fs.get failed")
		ses.sendStatement(fmt.Sprintf("550 Could not get file: %s.", err))
		return false
	}

	dc := ses.lastDataChanneler
	ses.lastDataChanneler = nil // dc in use!

	dc.Sink(func(w io.Writer, r io.Reader) error {
		defer dc.Close()

		file, err := f.Read(rest)
		if err != nil {
			log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processRETR fs.File.Get failed")
			ses.sendStatement(fmt.Sprintf("550 Could not get file: %s.", err))
			return err
		}
		defer file.Close()

		buf := make([]byte, 1024*256)

		ses.sendStatement(fmt.Sprintf("150 Opening BINARY mode data connection for %s.", f.Name()))

		log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "f.FullPath()": f.FullPath(), "f.Size()": f.Size()}).Info("session::Session::processRETR transfer starting")

		for {
			iRead, err := file.Read(buf)

			log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "read": iRead, "f.Size()": f.Size()}).Debug("session::Session::processRETR transfer starting")

			if err != nil {
				if err == io.EOF {
					// Flush buffer
					iWritten, err := w.Write(buf[0:iRead])
					if err != nil {
						// something went south :(
						log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processRETR socket.Send failed")
						return err
					}
					log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "sent": iWritten, "f.Size()": f.Size()}).Debug("session::Session::processRETR transfer starting")

					// done
					log.WithFields(log.Fields{"ses": ses, "tokens": tokens}).Info("session::Session::processRETR transfer completed")
					ses.sendStatement("226 File send OK.")
					return nil
				}

				// something went south :(
				log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processRETR file.Read failed")
				return err
			}

			iWritten, err := w.Write(buf[0:iRead])
			if err != nil {
				// something went south :(
				log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processRETR socket.Send failed")
				return err
			}
			log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "sent": iWritten, "f.Size()": f.Size()}).Debug("session::Session::processRETR transfer starting")
		}
	})

	return false
}

func (ses *Session) processSTOR(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "STOR"}).Info("session::Session::processSTOR method begin")

	if len(tokens) < 2 {
		ses.sendStatement("501 object needed!")
		return false
	}

	f, err := ses.fileProvider.New(strings.Join(tokens[1:], " "), false)

	if err != nil {
		log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processSTOR fs.New failed")
		ses.sendStatement(fmt.Sprintf("550 Could not create file file: %s.", err))
		return false
	}

	dc := ses.lastDataChanneler
	ses.lastDataChanneler = nil // dc in use!

	dc.Sink(func(w io.Writer, r io.Reader) error {
		defer dc.Close()

		file, err := f.Write()
		if err != nil {
			log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processSTOR fs.File.Write failed")
			ses.sendStatement(fmt.Sprintf("550 Could not get file: %s.", err))
			return err
		}
		defer file.Close()

		buf := make([]byte, 1024*1024*100)

		ses.sendStatement(fmt.Sprintf("150 Opening BINARY mode data connection for %s.", f.Name()))

		log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "f.FullPath()": f.FullPath(), "f.Size()": f.Size()}).Info("session::Session::processSTOR transfer starting")

		for {
			iRead, err := r.Read(buf)
			if err != nil {
				if err == io.EOF {
					// done
					log.WithFields(log.Fields{"ses": ses, "tokens": tokens}).Info("session::Session::processSTOR transfer completed")
					ses.sendStatement("226 File received OK.")
					return nil
				}

				// something went south :(
				log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processSTOR socket.Read failed")
				return err
			}

			_, err = file.Write(buf[0:iRead])
			if err != nil {
				// something went south :(
				log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processSTOR file.Write failed")
				return err
			}
		}
	})

	return false
}

func (ses *Session) processLIST(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "LIST"}).Info("session::Session::processLIST method begin")

	lastCWD := ses.fileProvider.CurrentDirectory()

	if len(tokens) > 1 {
		if err := ses.fileProvider.ChangeDirectory(tokens[1]); err != nil {
			ses.sendStatement(fmt.Sprintf("451 cannot retrieve directory list: %s", err))
			return false
		}
	}

	files, err := ses.fileProvider.List()

	if err != nil {
		ses.sendStatement(fmt.Sprintf("451 cannot retrieve directory list: %s", err))
		return false
	}

	if len(tokens) > 1 {
		if err := ses.fileProvider.ChangeDirectory(lastCWD); err != nil {
			ses.sendStatement(fmt.Sprintf("451 cannot retrieve directory list: %s", err))
			return false
		}
	}

	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "LIST", "len(files)": len(files)}).Info("session::Session::processLIST method after ses.fileProvider.List()")

	// prepare directory listing
	buf := new(bytes.Buffer)
	{
		{ // send . and ..
			str := fmt.Sprintf("%s   1 %-10s %-10s %10d Jan  02  2006 %s\r\n", "drwxrwxrwx", "group", "user", 0, ".")
			buf.WriteString(str)
			str = fmt.Sprintf("%s   1 %-10s %-10s %10d Jan  02  2006 %s\r\n", "drwxrwxrwx", "group", "user", 0, "..")
			buf.WriteString(str)
		}

		for _, file := range files {
			var date string
			diff := time.Now().Sub(file.ModTime())
			if diff.Hours() > 24*30*6 {
				date = fmt.Sprintf("%3.3s %2d  %04d", file.ModTime().Month(), file.ModTime().Day(), file.ModTime().Year())
			} else {
				date = fmt.Sprintf("%3.3s %2d %02d:%02d", file.ModTime().Month(), file.ModTime().Day(), file.ModTime().Hour(), file.ModTime().Minute())
			}

			str := fmt.Sprintf("%s   1 %-10s %-10s %10d %s %s\r\n", file.Mode(), "group", "user", file.Size(), date, file.Name())
			buf.WriteString(str)
		}
	}

	dc := ses.lastDataChanneler
	ses.lastDataChanneler = nil // dc in use!

	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "LIST", "len(files)": len(files)}).Info("session::Session::processLIST method after ses.lastDataChanneler = nil")

	dc.Sink(func(w io.Writer, r io.Reader) error {
		defer dc.Close()

		log.WithFields(log.Fields{"w": w, "string(buf.Bytes())": string(buf.Bytes())}).Debug("session::Session::processLIST::anonymous sending directory list")

		ses.sendStatement("150 Here comes the directory listing.")

		_, err := w.Write(buf.Bytes())

		if err != nil {
			ses.sendStatement(fmt.Sprintf("550 Directory listing error: %s", err))
			return err
		}

		ses.sendStatement("226 Directory send OK.")
		return nil
	})

	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "LIST"}).Info("session::Session::processLIST method end with success")
	return false
}

func (ses *Session) processNLST(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "NLST"}).Info("session::Session::processNLST method begin")

	lastCWD := ses.fileProvider.CurrentDirectory()

	if len(tokens) > 1 {
		if err := ses.fileProvider.ChangeDirectory(tokens[1]); err != nil {
			ses.sendStatement(fmt.Sprintf("451 cannot retrieve directory list: %s", err))
			return false
		}
	}

	files, err := ses.fileProvider.List()

	if err != nil {
		ses.sendStatement(fmt.Sprintf("451 cannot retrieve directory list: %s", err))
		return false
	}

	if len(tokens) > 1 {
		if err := ses.fileProvider.ChangeDirectory(lastCWD); err != nil {
			ses.sendStatement(fmt.Sprintf("451 cannot retrieve directory list: %s", err))
			return false
		}
	}

	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "NLST", "len(files)": len(files)}).Info("session::Session::processNLST method after ses.fileProvider.List()")

	// prepare directory listing
	buf := new(bytes.Buffer)
	for _, file := range files {
		str := fmt.Sprintf("%s\r\n", file.Name())
		buf.WriteString(str)
	}

	dc := ses.lastDataChanneler
	ses.lastDataChanneler = nil // dc in use!

	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "NLST", "len(files)": len(files)}).Info("session::Session::processNLST method after ses.lastDataChanneler = nil")

	dc.Sink(func(w io.Writer, r io.Reader) error {
		defer dc.Close()

		log.WithFields(log.Fields{"w": w, "string(buf.Bytes())": string(buf.Bytes())}).Debug("session::Session::processNLST::anonymous sending directory list")

		ses.sendStatement("150 Here comes the directory listing.")

		_, err := w.Write(buf.Bytes())

		if err != nil {
			ses.sendStatement(fmt.Sprintf("550 Directory listing error: %s", err))
			return err
		}

		ses.sendStatement("226 Directory send OK.")
		return nil
	})

	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "NLST"}).Info("session::Session::processNLST method end with success")
	return false
}

func (ses *Session) processUSER(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "USER"}).Info("session::Session::processUSER method begin")
	if len(tokens) < 2 {
		ses.sendStatement("501 user needed!")
		return false
	}

	ses.id.SetUsername(tokens[1])
	ses.sendStatement(fmt.Sprintf("331 Password required for %s.", ses.id.Username()))
	return false
}

func (ses *Session) processPASS(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "PASS"}).Info("session::Session::processPASS method begin")
	if len(tokens) < 2 {
		ses.sendStatement("501 password needed!")
		return false
	}

	password := tokens[1]

	if !ses.authFunc(ses.id.Username(), password) {
		ses.id.SetAuthenticated(false)
		ses.id.SetUsername("")
		ses.sendStatement("530 Password Rejected")
		return false
	}

	ses.id.SetAuthenticated(true)
	ses.sendStatement(fmt.Sprintf("230 User %s logged in.", ses.id.Username()))
	return false
}

func (ses *Session) processPASV(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "PASV"}).Info("session::Session::processPASV method begin")
	ip, err := getLocalIP()
	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 Could not get local IP: %s", err))
		return false
	}

	log.WithFields(log.Fields{"ip": ip.String()}).Debug("session::Session::processPASV local IP retrieved")

	err = ses.retrievePassivePort()
	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 Could not allocate passive port: %s", err))
		return false
	}

	log.WithFields(log.Fields{"ses.lastDataChanneler": ses.lastDataChanneler}).Debug("session::Session::processPASV passive port allotted")

	s := strings.Replace(ip.String(), ".", ",", -1)

	err = ses.lastDataChanneler.Open()
	if err != nil {
		log.WithFields(log.Fields{"ses.lastDataChanneler": ses.lastDataChanneler, "err": err}).Warn("session::Session::processPASV could not open passive port")
		ses.sendStatement(fmt.Sprintf("550 Could not open passive port: %s", err))
		return false
	}

	ses.sendStatement(fmt.Sprintf("%d Entering Passive Mode (%s,%s)", 227, s, ses.lastDataChanneler.ToPASVStringPort()))
	return false
}

func (ses *Session) processEPSV(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "EPSV"}).Info("session::Session::processEPSV method begin")

	err := ses.retrievePassivePort()
	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 Could not allocate passive port: %s", err))
		return false
	}

	log.WithFields(log.Fields{"ses.lastDataChanneler": ses.lastDataChanneler}).Debug("session::Session::processPASV passive port allotted")

	err = ses.lastDataChanneler.Open()
	if err != nil {
		log.WithFields(log.Fields{"ses.lastDataChanneler": ses.lastDataChanneler, "err": err}).Warn("session::Session::processPASV could not open passive port")
		ses.sendStatement(fmt.Sprintf("550 Could not open passive port: %s", err))
		return false
	}

	ses.sendStatement(fmt.Sprintf("229 Entering Extended Passive Mode (|||%d|)", ses.lastDataChanneler.Port()))
	return false
}

func (ses *Session) processTYPE(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "TYPE"}).Info("session::Session::processTYPE method begin")

	if len(tokens) < 2 {
		ses.sendStatement("501 type needed!")
		return false
	}

	if strings.ToLower(tokens[1]) != "i" && strings.ToLower(tokens[1]) != "a" {
		ses.sendStatement(fmt.Sprintf("504 Type I and A are the only one supported. %s is not supported at this time", tokens[1]))
		return false
	}

	ses.sendStatement(fmt.Sprintf("200 Type set to %s", strings.ToUpper(tokens[1])))
	return false
}

func (ses *Session) processSIZE(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "SIZE"}).Info("session::Session::processSIZE method begin")

	if len(tokens) < 2 {
		ses.sendStatement("501 object needed!")
		return false
	}

	file := clearPath(strings.Join(tokens[1:], " "))

	f, err := ses.fileProvider.Get(file)
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "SIZE", "file": file, "f": f, "err": err}).Debug("session::Session::processSIZE method after ses.fileProvider.Get(file)")

	if err != nil {
		log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "err": err}).Warn("session::Session::processSIZE fs.get failed")
		ses.sendStatement(fmt.Sprintf("550 Could not get file: %s.", err))
		return false
	}

	ses.sendStatement(fmt.Sprintf("213 %d", f.Size()))
	return false
}

func (ses *Session) processMKD(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "MKD"}).Info("session::Session::processMKD method begin")

	if len(tokens) < 1 {
		// either root or containter
		ses.sendStatement("501 folder name needed")
		return false
	}

	path := strings.Join(tokens[1:], " ")
	err := ses.fileProvider.CreateDirectory(path)

	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 cannot create folder %s (%s)", path, err))
		return false
	}

	dir, err := ses.fileProvider.Get(path)
	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 cannot create folder %s (%s)", path, err))
		return false
	}

	ses.sendStatement(fmt.Sprintf("257 \"%s\" directory created", dir.FullPath()))

	return false
}

func (ses *Session) processRMD(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "RMD"}).Info("session::Session::processRMD method begin")

	if len(tokens) < 1 {
		// either root or containter
		ses.sendStatement("501 folder name needed")
		return false
	}

	err := ses.fileProvider.RemoveDirectory(strings.Join(tokens[1:], " "))
	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 cannot delete folder %s (%s)", tokens[1], err))
		return false
	}

	ses.sendStatement("250 folder deleted successfully")

	return false
}

func (ses *Session) processDELE(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "DELE"}).Info("session::Session::processDELE method begin")

	if len(tokens) < 1 {
		// either root or containter
		ses.sendStatement("501 file name needed")
		return false
	}

	f, err := ses.fileProvider.Get(strings.Join(tokens[1:], " "))
	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 cannot delete file %s (%s)", tokens[1], err))
		return false
	}

	err = f.Delete()
	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 cannot delete file %s (%s)", tokens[1], err))
		return false
	}

	ses.sendStatement("200 file delete successfully")

	return false
}

func (ses *Session) processREST(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "REST"}).Info("session::Session::processREST method begin")

	if len(tokens) < 1 {
		// either root or containter
		ses.sendStatement("501 size needed")
		return false
	}

	_, err := fmt.Sscanf("%d", tokens[1], &ses.lastREST)
	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 syntax error (%s)", err))
		return false
	}

	ses.sendStatement("350 start position moved successfully")

	return false
}

func (ses *Session) processAUTH(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "AUTH"}).Info("session::Session::processAUTH method begin")

	if ses.cert == nil || ses.conn.IsSecure() { // one does not need AUTH if is already encrypted
		ses.sendStatement("502 not supported")
		return false
	}

	if len(tokens) < 2 {
		ses.sendStatement("550 must specify protocol!")
		return false
	}

	if tokens[1] != "TLS" {
		ses.sendStatement(fmt.Sprintf("503 %s is not supported", tokens[0]))
		return false
	}
	ses.sendStatement("234 Using authentication type TLS")

	err := ses.conn.SwitchToTLS()
	if err != nil {
		ses.sendStatement(fmt.Sprintf("550 error initializing TLS: %s", err))
		return false
	}

	return false
}

func (ses *Session) processPROT(tokens []string) bool {
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "PROT"}).Info("session::Session::processPROT method begin")

	if !ses.conn.IsSecure() { // PROT needs command channel encryption in place
		ses.sendStatement("502 not supported")
		return false
	}

	if len(tokens) < 2 {
		ses.sendStatement("550 must specify protection level!")
		return false
	}

	protLevel := strings.ToUpper(tokens[1])
	log.WithFields(log.Fields{"ses": ses, "tokens": tokens, "command": "PROT", "protLevel": protLevel}).Info("session::Session::processPROT method validating protection level")

	if protLevel == "P" {
		ses.dataChannelEncryption = true
		if ses.lastDataChanneler != nil {
			ses.lastDataChanneler.SetEncrypted(true)
		}
		ses.sendStatement("200 data channel TLS encryption enabled")
		return false
	}
	if protLevel == "C" {
		ses.dataChannelEncryption = false
		if ses.lastDataChanneler != nil {
			ses.lastDataChanneler.SetEncrypted(false)
		}
		ses.sendStatement("200 data channel TLS encryption disabled")
		return false
	}

	ses.sendStatement(fmt.Sprintf("550 %s is not supported", protLevel))
	return false
}
