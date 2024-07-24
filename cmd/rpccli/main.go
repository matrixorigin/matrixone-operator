package main

import (
	"context"
	"github.com/matrixorigin/matrixone-operator/pkg/querycli"
	"go.uber.org/zap"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"time"
)

func main() {
	zapLogger := logzap.NewRaw()
	qc, err := querycli.New(zapLogger.Named("querycli"))
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := qc.ShowProcessList(ctx, "localhost:6004")
	if err != nil {
		panic(err)
	}
	zapLogger.Info("resp", zap.Any("resp", resp))
}
