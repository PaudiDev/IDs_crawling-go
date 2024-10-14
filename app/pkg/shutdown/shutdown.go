package shutdown

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func HandleSIGTERM(cancel context.CancelFunc) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c

	cancel()
	slog.Error("Keyboard Interrupt Received (SIGTERM): Stopped program and aborted all requests.")
	Shutdown()
}

func Shutdown() {
	time.Sleep(250 * time.Millisecond)
	os.Exit(1)
}
