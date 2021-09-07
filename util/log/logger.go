/**
 * @Author: vincent
 * @Description:
 * @File:  logger
 * @Version: 1.0.0
 * @Date: 2021/9/3 14:39
 */

package log

import "go.uber.org/zap"

var Logger *zap.SugaredLogger

func Init(mode string) {
	var l *zap.Logger
	if mode == "dev" {
		l, _ = zap.NewDevelopment()
	} else {
		l, _ = zap.NewProduction()
	}

	Logger = l.Sugar()
}
