// Package main is the sample application.
// Windows users should be aware that the Windows firewall
// might block the socket creation. To avoid this, insert
// the executable in the exclusion list
package main

import (
	"crypto/tls"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/mindflavor/ftpserver2/ftp"
	"github.com/mindflavor/ftpserver2/ftp/fs"
	"github.com/mindflavor/ftpserver2/ftp/fs/azure"
	"github.com/mindflavor/ftpserver2/ftp/fs/localFS"

	"github.com/rifflock/lfshook"
)

// example go install github.com/mindflavor/ftpserver2 && %GOPATH%\bin\ftpserver2 -lfs C:\temp -ll Debug -lDebug D:\temp\ftp.log -lInfo D:\temp\ftp.log -lWarn D:\temp\ftp.log -lError D:\temp\ftp.log -crt C:\temp\cert.pem -key C:\temp\key.pem

func main() {
	authFunc := func(username, password string) bool {
		log.WithFields(log.Fields{"username": username, "password": "xxx"}).Debug("main::authFunc Authentication requested")
		return true
	}

	logLevel := flag.String("ll", "Info", "Minimum log level. Available values are Debug, Info, Warn, Error")
	azureAccount := flag.String("an", "", "Azure blob storage account name")
	azureKey := flag.String("ak", "", "Azure blob storage account key (either primary or secondary)")
	localFSRoot := flag.String("lfs", "", "Local file system root")

	tlsCertFile := flag.String("crt", "", "TLS certificate file")
	tlsKeyFile := flag.String("key", "", "TLS certificate key file")

	plainCmdPort := flag.Int("plainPort", 21, "Plain FTP port (unencrypted). If you specify a TLS certificate and key encryption you can pass -1 to start a SFTP implicit server only")
	encrCmdPort := flag.Int("tlsPort", 990, "Encrypted FTP port. If you do not specify a TLS certificate this port is ignored. If you specify -1 the implicit SFTP is disabled")

	lowerPort := flag.Int("minPasvPort", 50000, "Lower passive port range")
	higerPort := flag.Int("maxPasvPort", 50100, "Higher passive port range")

	logFileDebug := flag.String("lDebug", "", "Debug level log file")
	logFileInfo := flag.String("lInfo", "", "Info level log file")
	logFileWarn := flag.String("lWarn", "", "Warn level log file")
	logFileError := flag.String("lError", "", "Error level log file")

	flag.Parse()

	if *logFileDebug != "" || *logFileInfo != "" || *logFileWarn != "" || *logFileError != "" {
		log.AddHook(lfshook.NewHook(lfshook.PathMap{
			log.DebugLevel: *logFileDebug,
			log.InfoLevel:  *logFileInfo,
			log.WarnLevel:  *logFileWarn,
			log.ErrorLevel: *logFileError,
		}))
	}

	if (*azureAccount == "" || *azureKey == "") && *localFSRoot == "" {
		log.Error("main::main must specify either a local file system root or a azure account (both name and key) as storage. Check the command line docs for help")
		os.Exit(-1)
	}

	switch strings.ToLower(*logLevel) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.WithFields(log.Fields{"logLevel": logLevel}).Error("main::main unsupported log level")
		os.Exit(-1)
	}

	log.WithField("program", os.Args[0]).Info("Program started")

	var fs fs.FileProvider
	var err error

	var cert tls.Certificate
	if *tlsCertFile != "" && *tlsKeyFile != "" {
		cert, err = tls.LoadX509KeyPair(*tlsCertFile, *tlsKeyFile)
		if err != nil {
			panic(err)
		}
	}

	if *azureAccount != "" && *azureKey != "" {
		log.WithFields(log.Fields{"account": *azureAccount}).Info("main::main initializating Azure blob storage backend")
		fs, err = azureFS.New(*azureAccount, *azureKey)
	} else {
		log.WithFields(log.Fields{"localFSRoot": *localFSRoot}).Info("main::main initializating local fs backend")
		fs, err = localFS.New(*localFSRoot)
	}

	if err != nil {
		panic(err)
	}

	// 15 min connection timeout
	timeout, err := time.ParseDuration("15m")
	if err != nil {
		panic(err)
	}

	var srv *ftp.Server
	if *tlsCertFile != "" && *tlsKeyFile != "" {
		if *encrCmdPort == -1 {
			srv = ftp.NewTLS(*plainCmdPort, &cert, timeout, *lowerPort, *higerPort, authFunc, fs)
		} else {
			srv = ftp.New(*plainCmdPort, *encrCmdPort, &cert, timeout, *lowerPort, *higerPort, authFunc, fs)
		}
	} else {
		srv = ftp.NewPlain(*plainCmdPort, nil, timeout, *lowerPort, *higerPort, authFunc, fs)
	}

	srv.Accept()

	signal_chan := make(chan os.Signal, 1)
	var code int
	signal.Notify(signal_chan)
	for {
		s := <-signal_chan
		switch s {
		case syscall.SIGINT:
			log.WithFields(log.Fields{"signal": "SIGINT"}).Warn("main::main " + s.String())
			code = 0
		case syscall.SIGTERM:
			log.WithFields(log.Fields{"signal": "SIGTERM"}).Warn("main::main " + s.String())
			code = 0
		case syscall.SIGPIPE:
			log.WithFields(log.Fields{"signal": "SIGPIPE"}).Warn("main::main " + s.String())
			continue
		default:
			log.Error("main::main Unknown signal (" + s.String() + ")")
			code = 1
		}
		break
	}
	os.Exit(code)
}
