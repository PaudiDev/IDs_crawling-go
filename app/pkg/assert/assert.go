package assert

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	"crawler/app/pkg/shutdown"
	"crawler/app/pkg/utils/mapx"
)

type AssertData mapx.BasicMap

var ctxCancel context.CancelFunc = nil

// This function should be called as soon as the context is created
func LoadCtxCancel(cancel context.CancelFunc) {
	ctxCancel = cancel
}

func runAssert(msg string, dataArgs ...AssertData) {
	slogData := AssertData{"msg": msg}
	for _, data := range dataArgs {
		duplicateKeys := mapx.CopyNoDuplicates(
			(mapx.BasicMap)(data),
			(mapx.BasicMap)(slogData),
		)

		for _, dk := range duplicateKeys {
			fmt.Fprintf(os.Stderr, "WARNING: Duplicate key %s. Renaming to %s\n", dk, dk+"_")
		}
	}

	assertErr := fmt.Sprintf("ARGS: %+v\n", dataArgs)
	assertErr += "ASSERT\n"
	for k, v := range slogData {
		assertErr += fmt.Sprintf("%s: %v\n", k, v)
	}
	assertErr += string(debug.Stack())

	if ctxCancel != nil {
		ctxCancel()
	}
	slog.Error(assertErr)
	shutdown.Shutdown()
}

func Assert(truth bool, msg string, dataArgs ...AssertData) {
	if !truth {
		runAssert(msg, dataArgs...)
	}
}

func Nil(item any, msg string, dataArgs ...AssertData) {
	if item != nil {
		slog.Error("Nil#not nil encountered")
		runAssert(msg, dataArgs...)
	}
}

func NotNil(item any, msg string, dataArgs ...AssertData) {
	if item == nil {
		slog.Error("NotNil#nil encountered")
		runAssert(msg, dataArgs...)
	}
}

func Never(msg string, dataArgs ...AssertData) {
	slog.Error("Never#never encountered")
	runAssert(msg, dataArgs...)
}

func NoError(err error, msg string, dataArgs ...AssertData) {
	if err != nil {
		slog.Error("NoError#error encountered", "error", err)
		dataArgs = append(dataArgs, AssertData{"error": err})
		runAssert(msg, dataArgs...)
	}
}
