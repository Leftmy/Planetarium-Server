package logger

import "log"

// Init initializes the application logger (e.g., Zap or Logrus).
// For now, it's a stub using the standard library.
func Init() {
	log.Println("Logger initialized successfully")
}
