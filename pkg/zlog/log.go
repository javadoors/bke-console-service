/*
 * Copyright (c) 2025Huawei Technologies Co., Ltd.
 * openFuyao is licensed under Mulan PSL v2.
 * You can use this software according to the terms and conditions of the Mulan PSL v2.
 * You may obtain a copy of Mulan PSL v2 at:
 *          http://license.coscl.org.cn/MulanPSL2
 * THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
 * EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
 * MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
 * See the Mulan PSL v2 for more details.
 */

package zlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultConfigPath = "/etc/console-service/log-config"
	defaultConfigName = "console-service"
	defaultConfigType = "yaml"
	defaultLogPath    = "/var/log"
)

// Logger logger
var Logger *zap.SugaredLogger

var logLevel = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
}

var watchOnce = sync.Once{}

// LogConfig log config
type LogConfig struct {
	Level       string
	EncoderType string
	Path        string
	FileName    string
	MaxSize     int
	MaxBackups  int
	MaxAge      int
	LocalTime   bool
	Compress    bool
	OutMod      string
}

func init() {
	var conf *LogConfig
	var err error
	if conf, err = loadConfig(); err != nil {
		fmt.Printf("loadConfig fail err is %v. use DefaultConf\n", err)
		conf = getDefaultConf()
	}
	Logger = GetLogger(conf)
}

func loadConfig() (*LogConfig, error) {
	viper.AddConfigPath(defaultConfigPath)
	viper.SetConfigName(defaultConfigName)
	viper.SetConfigType(defaultConfigType)

	// 添加当前根目录，仅用于debug，打包构建时请勿开启
	config, err := parseConfig()
	if err != nil {
		return nil, err
	}
	watchConfig()
	return config, nil
}

func getDefaultConf() *LogConfig {
	var defaultConf = &LogConfig{
		Level:       "info",
		EncoderType: "console",
		Path:        defaultLogPath,
		FileName:    "root.log",
		MaxSize:     20,
		MaxBackups:  5,
		MaxAge:      30,
		LocalTime:   false,
		Compress:    true,
		OutMod:      "both",
	}
	exePath, err := os.Executable()
	if err != nil {
		return defaultConf
	}
	// 获取运行文件名称，作为/var/log目录下的子目录
	serviceName := strings.TrimSuffix(filepath.Base(exePath), filepath.Ext(filepath.Base(exePath)))
	defaultConf.Path = filepath.Join(defaultLogPath, serviceName)
	return defaultConf
}

// GetLogger get logger function
func GetLogger(conf *LogConfig) *zap.SugaredLogger {
	writeSyncer := getLogWriter(conf)
	encoder := getEncoder(conf)
	level, ok := logLevel[strings.ToLower(conf.Level)]
	if !ok {
		level = logLevel["info"]
	}
	core := zapcore.NewCore(encoder, writeSyncer, level)
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return logger.Sugar()
}

func watchConfig() {
	// 监听配置文件的变化
	watchOnce.Do(func() {
		viper.WatchConfig()
		viper.OnConfigChange(func(e fsnotify.Event) {
			Logger.Warn("Config file changed")
			// 重新加载配置
			conf, err := parseConfig()
			if err != nil {
				Logger.Warnf("Error reloading config file: %v\n", err)
			} else {
				Logger = GetLogger(conf)
			}
		})
	})
}

func parseConfig() (*LogConfig, error) {
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}
	var config LogConfig
	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// //获取编码器,NewJSONEncoder()输出json格式，NewConsoleEncoder()输出普通文本格式
func getEncoder(conf *LogConfig) zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	// 指定时间格式 for example: 2021-09-11t20:05:54.852+0800
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// 按级别显示不同颜色，不需要的话取值zapcore.CapitalLevelEncoder就可以了
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// NewJSONEncoder()输出json格式，NewConsoleEncoder()输出普通文本格式
	if strings.ToLower(conf.EncoderType) == "json" {
		return zapcore.NewJSONEncoder(encoderConfig)
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getLogWriter(conf *LogConfig) zapcore.WriteSyncer {
	// 只输出到控制台
	if conf.OutMod == "console" {
		return zapcore.AddSync(os.Stdout)
	}
	// 日志文件配置
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filepath.Join(conf.Path, conf.FileName),
		MaxSize:    conf.MaxSize,
		MaxBackups: conf.MaxBackups,
		MaxAge:     conf.MaxAge,
		LocalTime:  conf.LocalTime,
		Compress:   conf.Compress,
	}
	if conf.OutMod == "both" {
		// 控制台和文件都输出
		return zapcore.NewMultiWriteSyncer(zapcore.AddSync(lumberJackLogger), zapcore.AddSync(os.Stdout))
	}
	if conf.OutMod == "file" {
		// 只输出到文件
		return zapcore.AddSync(lumberJackLogger)
	}
	return zapcore.AddSync(os.Stdout)
}

// With logs a with-level message
func With(arguments ...interface{}) *zap.SugaredLogger {
	return Logger.With(arguments...)
}

// Error logs an error-level message
func Error(arguments ...interface{}) {
	Logger.Error(arguments...)
}

// Warn logs a warn-level message
func Warn(arguments ...interface{}) {
	Logger.Warn(arguments...)
}

// Info logs an info-level message
func Info(arguments ...interface{}) {
	Logger.Info(arguments...)
}

// Debug logs a debug-level message
func Debug(arguments ...interface{}) {
	Logger.Debug(arguments...)
}

// Fatal logs a fatal-level message
func Fatal(arguments ...interface{}) {
	Logger.Fatal(arguments...)
}

// Errrorf logs an errorf-level message
func Errorf(template string, arguments ...interface{}) {
	Logger.Errorf(template, arguments...)
}

// Warnf logs a warnf-level message
func Warnf(template string, args ...interface{}) {
	Logger.Warnf(template, args...)
}

// Infof logs a infof-level message
func Infof(template string, arguments ...interface{}) {
	Logger.Infof(template, arguments...)
}

// Debugf logs a debugf-level message
func Debugf(template string, arguments ...interface{}) {
	Logger.Debugf(template, arguments...)
}

// Fatalf logs a fatalf-level message
func Fatalf(template string, arguments ...interface{}) {
	Logger.Fatalf(template, arguments...)
}

// Sync logs a sync-level message
func Sync() error {
	return Logger.Sync()
}
