package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/go-stack/stack"
)

const (
	TimeFormateUtcMs = "2006-01-02T15:04:05.000000Z07:00"
	TimeFormateUtc   = "2006-01-02T15:04:05Z07:00"
	TimeFormateZ     = "2006-01-02T15:04:05.000000Z"
	TimeFormate8     = "2006-01-02T15:04:05.000"
	TimeFormate8_2   = "2006-01-02 15:04:05"
)

type bufferPool struct {
	*sync.Pool
}

var BufPool = newBufferPool(512)

func newBufferPool(size int) *bufferPool {
	return &bufferPool{
		&sync.Pool{New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, size))
			// return &bytes.Buffer{}
		}},
	}
}

func (bp *bufferPool) Get() *bytes.Buffer {
	return (bp.Pool.Get()).(*bytes.Buffer)
}

func (bp *bufferPool) Put(b *bytes.Buffer) {
	b.Reset()
	bp.Pool.Put(b)
}

// NowUnixMs 返回当前毫秒数
func NowUnixMs() int64 {
	tn := time.Now()
	return tn.UnixNano() / 1e6
}

func FormateTimeUtcToZ(time string) string {
	times := strings.Split(time, "+")

	if len(times) > 1 {
		return times[0] + "Z"
	} else {
		return time
	}
}

func FormateTimeZ(t time.Time) string {
	t = t.UTC()
	return t.Format(TimeFormateZ)
}

func ParseTimeUtc(timestr string) time.Time {
	t, err := time.Parse(TimeFormateUtc, timestr)
	if err != nil {
		t, err = time.Parse(TimeFormateUtcMs, timestr)
		if err != nil {
			logrus.WithError(err).Errorf("parse time err")
			return time.Time{}
		}
	}
	return t
}

func ParseTimestampUtc(timestr string) int64 {
	t, err := time.Parse(TimeFormateUtc, timestr)
	if err != nil {
		t, err = time.Parse(TimeFormateUtcMs, timestr)
		if err != nil {
			logrus.WithError(err).Errorf("parse time err")
			return 0
		}
	}
	return t.Unix()
}

func Add10MinUtc(timestr string) string {
	t, err := time.Parse(TimeFormateUtcMs, timestr)
	if err != nil {
		t, err = time.Parse(TimeFormateUtc, timestr)
		if err != nil {
			logrus.WithError(err).Errorf("parse time err")
			return ""
		}
	}

	return t.Add(10 * time.Minute).Format(TimeFormateUtcMs)
}

func Sub10MinUtc(timestr string) string {
	t, err := time.Parse(TimeFormateUtcMs, timestr)
	if err != nil {
		t, err = time.Parse(TimeFormateUtc, timestr)
		if err != nil {
			logrus.WithError(err).Errorf("parse time err")
			return ""
		}
	}

	return t.Add(-10 * time.Minute).Format(TimeFormateUtcMs)
}

func ParseTimestampZ(timestr string) int64 {
	t, err := time.Parse(TimeFormateZ, timestr)
	if err != nil {
		logrus.WithError(err).Errorf("parse time err")
		return 0
	}
	return t.Unix()
}

func ParseTimestamp8(timestr string) int64 {
	utc8, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		logrus.WithError(err).Error("load location err")
		return 0
	}
	t, err := time.ParseInLocation(TimeFormate8, timestr, utc8)
	if err != nil {
		t, err = time.ParseInLocation(TimeFormate8_2, timestr, utc8)
		if err == nil {
			return t.Unix()
		}
		logrus.WithError(err).Errorf("parse time err")
		return 0
	}
	return t.Unix()
}

func WaitForIntegerMinute() {
	time.Sleep(time.Duration((60 - time.Now().Second()%60)) * time.Second)
}

func WaitForHalfMinute() {
	time.Sleep(time.Duration((30 - time.Now().Second()%30)) * time.Second)
}

// TruncateID returns a shorthand version of a string identifier for convenience.
// A collision with other shorthands is very unlikely, but possible.
// In case of a collision a lookup with TruncIndex.Get() will fail, and the caller
// will need to use a langer prefix, or the full-length Id.
func TruncateID(id string) string {
	shortLen := 12
	if len(id) < shortLen {
		shortLen = len(id)
	}
	return id[:shortLen]
}

// GenerateRandomID returns an unique id
func GenerateRandomID(num int) string {
	for {
		id := make([]byte, num/2)
		if _, err := io.ReadFull(rand.Reader, id); err != nil {
			panic(err) // This shouldn't happen
		}
		value := hex.EncodeToString(id)
		// if we try to parse the truncated for as an int and we don't have
		// an error then the value is all numberic and causes issues when
		// used as a hostname. ref #3869
		if _, err := strconv.ParseInt(TruncateID(value), 10, 64); err == nil {
			continue
		}
		return value
	}
}

func GenerateRandomUint32() uint32 {
	id := make([]byte, 4)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		panic(err)
	}
	return binary.BigEndian.Uint32(id)
}

func GenerateRandomUint64() uint64 {
	id := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		panic(err)
	}
	return binary.BigEndian.Uint64(id)
}

func Uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

func Uint16ToBytes(n uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, n)
	return b
}

// var (
// 	dunno     = []byte("???")
// 	centerDot = []byte("·")
// 	dot       = []byte(".")
// 	slash     = []byte("/")
// )

// // source returns a space-trimmed slice of the n'th line.
// func source(lines [][]byte, n int) []byte {
// 	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
// 	if n < 0 || n >= len(lines) {
// 		return dunno
// 	}
// 	return bytes.TrimSpace(lines[n])
// }

// // function returns, if possible, the name of the function containing the PC.
// func function(pc uintptr) []byte {
// 	fn := runtime.FuncForPC(pc)
// 	if fn == nil {
// 		return dunno
// 	}
// 	name := []byte(fn.Name())
// 	// The name includes the path name to the package, which is unnecessary
// 	// since the file name is already included.  Plus, it has center dots.
// 	// That is, we see
// 	//	runtime/debug.*T·ptrmethod
// 	// and want
// 	//	*T.ptrmethod
// 	// Also the package path might contains dot (e.g. code.google.com/...),
// 	// so first eliminate the path prefix
// 	if lastslash := bytes.LastIndex(name, slash); lastslash >= 0 {
// 		name = name[lastslash+1:]
// 	}
// 	if period := bytes.Index(name, dot); period >= 0 {
// 		name = name[period+1:]
// 	}
// 	name = bytes.Replace(name, centerDot, dot, -1)
// 	return name
// }

// func stack(skip int) []byte {
// 	buf := new(bytes.Buffer) // the returned data
// 	// As we loop, we open files and read them. These variables record the currently
// 	// loaded file.
// 	var lines [][]byte
// 	var lastFile string
// 	for i := skip; ; i++ { // Skip the expected number of frames
// 		pc, file, line, ok := runtime.Caller(i)
// 		if !ok {
// 			break
// 		}
// 		// Print this much at least.  If we can't find the source, it won't show.
// 		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
// 		if file != lastFile {
// 			data, err := ioutil.ReadFile(file)
// 			if err != nil {
// 				continue
// 			}
// 			lines = bytes.Split(data, []byte{'\n'})
// 			lastFile = file
// 		}
// 		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
// 	}
// 	return buf.Bytes()
// }

func RecoverStack() {
	// err := recover()
	// if err != nil {
	// 	stack := stack(3)
	// 	logrus.Errorf("PANIC: %s\n%s", err, stack)
	// }
	err := recover()
	if err != nil {
		log.Printf("panic: %v\n", err)
		log.Println("Stack")
		cts := stack.Trace()
		for i := range cts {
			log.Printf("Stack: %+v %n", cts[i], cts[i])
		}
	}
}
