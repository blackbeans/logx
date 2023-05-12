package logx

import (
	"encoding/xml"
	"fmt"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

var loggers = make(map[string]*logrus.Logger)
var txtFormatter *prefixed.TextFormatter

func init() {
	txtFormatter = &prefixed.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02.15:04:05.000000",
		ForceFormatting: true,
		ForceColors:     true,
	}
}

type xmlProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

type xmlFilter struct {
	Enabled  string        `xml:"enabled,attr"`
	Tag      string        `xml:"tag"`
	Level    string        `xml:"level"`
	Type     string        `xml:"type"`
	Property []xmlProperty `xml:"property"`
}

type xmlLoggerConfig struct {
	Filter []xmlFilter `xml:"filter"`
}

// Load XML configuration; see conf/log.xml for documentation
func InitLogger(logPath string, filename string) {

	// Open the configuration file
	fd, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "InitLogger: Error: Could not open %q for reading: %s\n", filename, err)
		os.Exit(1)
	}

	contents, err := ioutil.ReadAll(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "InitLogger: Error: Could not read %q: %s\n", filename, err)
		os.Exit(1)
	}

	xc := new(xmlLoggerConfig)
	if err := xml.Unmarshal(contents, xc); err != nil {
		fmt.Fprintf(os.Stderr, "InitLogger: Error: Could not parse XML configuration in %q: %s\n", filename, err)
		os.Exit(1)
	}

	for _, xmlfilt := range xc.Filter {
		var filt = logrus.New()
		level, err := logrus.ParseLevel(xmlfilt.Level)
		if nil != err {
			panic(err)
		}
		filt.SetLevel(level)
		filt.SetFormatter(txtFormatter)
		switch xmlfilt.Type {
		case "console":
			filt.SetOutput(os.Stdout)
		case "file":
			output, err := xmlToFileLogWriter(filename, xmlfilt.Property)
			if nil == err {
				filt.SetOutput(output)
			}
		}
		loggers[xmlfilt.Tag] = filt
	}

	stderr, ok := loggers["stderr"]
	if ok {
		stderr = logrus.New()
		stderr.SetLevel(logrus.ErrorLevel)
		stderr.SetFormatter(txtFormatter)
		rotate, err := rotatelogs.New(
			path.Join(logPath, "stderr.log-%Y%m%d%H"),
			rotatelogs.WithLinkName(path.Join(logPath, "stderr.log")),
			rotatelogs.WithRotationTime(time.Hour),
		)
		if nil != err {
			panic(err)
		}
		stderr.SetOutput(rotate)
		loggers["stderr"] = stderr
	}

	//
	stdout, ok := loggers["stdout"]
	if ok {
		stderr = logrus.New()
		stderr.SetLevel(logrus.InfoLevel)
		stderr.SetFormatter(txtFormatter)
		rotate, err := rotatelogs.New(
			path.Join(logPath, "stdout.log-%Y%m%d%H"),
			rotatelogs.WithLinkName(path.Join(logPath, "stdout.log")),
			rotatelogs.WithRotationTime(time.Hour),
		)
		if nil != err {
			panic(err)
		}
		stderr.SetOutput(rotate)
		loggers["stdout"] = stderr
	}

	logrus.AddHook(lfshook.NewHook(lfshook.WriterMap{
		logrus.ErrorLevel: stderr.Out,
		logrus.PanicLevel: stderr.Out,
		logrus.FatalLevel: stderr.Out,
	}, txtFormatter))

	logrus.AddHook(lfshook.NewHook(lfshook.WriterMap{
		logrus.DebugLevel: stdout.Out,
		logrus.InfoLevel:  stdout.Out,
		logrus.WarnLevel:  stdout.Out,
	}, txtFormatter))
}

// Parse a number with K/M/G suffixes based on thousands (1000) or 2^10 (1024)
func strToNumSuffix(str string, mult int) int {
	num := 1
	if len(str) > 1 {
		switch str[len(str)-1] {
		case 'G', 'g':
			num *= mult
			fallthrough
		case 'M', 'm':
			num *= mult
			fallthrough
		case 'K', 'k':
			num *= mult
			str = str[0 : len(str)-1]
		}
	}
	parsed, _ := strconv.Atoi(str)
	return parsed * num
}
func xmlToFileLogWriter(filename string, props []xmlProperty) (io.Writer, error) {
	maxbackups := uint64(10)
	maxsize := strToNumSuffix("100M", 1024)

	// Parse properties
	for _, prop := range props {
		switch prop.Name {
		case "filename":
			filename = strings.Trim(prop.Value, " \r\n")
		case "maxbackups":
			maxbackups, _ = strconv.ParseUint(strings.Trim(prop.Value, " \r\n"), 10, 32)
		case "maxsize":
			maxsize = strToNumSuffix(strings.Trim(prop.Value, " \r\n"), 1024)
		}
	}

	rotate, err := rotatelogs.New(
		filename+"-%Y%m%d%H",
		rotatelogs.WithLinkName(filename),
		rotatelogs.WithRotationTime(time.Hour),
		rotatelogs.WithRotationCount(uint(maxbackups)),
		rotatelogs.WithRotationSize(int64(maxsize)),
	)
	if nil != err {
		logrus.Errorf("打开日志文件失败，默认输出到stderr")
		return nil, err
	} else {
		return rotate, nil
	}
}

func GetLogger(name string) *logrus.Logger {
	if l, ok := loggers[name]; ok {
		return l
	}
	return logrus.StandardLogger()
}
