package mylog

import (
	"os"
	"time"
	"github.com/onsi/ginkgo/v2"
	"github.com/sirupsen/logrus"
)

// TestLogger wraps logrus.Logger to include test context
type TestLogger struct {
	*logrus.Entry
}
// CustomFormatter implements logrus.Formatter interface
type CustomFormatter struct{}


// Format formats the log entry as desired
func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
        timestamp := entry.Time.Format(time.StampMilli)
        category, ok := entry.Data["category"].(string)
        if !ok {
                category = "unknown"
        }
        level := entry.Level.String()
        message := entry.Message

        formatted := []byte(timestamp + " [" + category + "][" + level + "][" + message + "]\n")
        return formatted, nil
}


// NewTestLogger initializes a logger with GinkgoWriter as the output
func NewTestLogger() *TestLogger {
	logger := logrus.New()
	//logger.SetFormatter(&logrus.JSONFormatter{})
	//logger.SetFormatter(&logrus.TextFormatter{})
	logger.SetFormatter(new(CustomFormatter)) // Set the custom formatter
	logger.SetOutput(os.Stdout)
	return &TestLogger{logger.WithField("message", ginkgo.CurrentSpecReport().LeafNodeText)}
}

// Info logs a message with the current test context
func (tl *TestLogger) Info(category, msg string) {
	tl.WithCategory(category).Info(msg)
}

func (tl *TestLogger) Infof(category, msg string, args ...interface{}) {
	tl.WithCategory(category).Infof(msg, args...)
}

// withCategory adds a log category to the entry
func (tl *TestLogger) WithCategory(category string) *logrus.Entry {
	return tl.WithField("category", category)
}

// Error logs an error-level message with a category (for failures)
func (tl *TestLogger) Error(category, msg string) {
	tl.WithCategory(category).Error(msg)
}
