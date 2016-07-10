package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/yazver/gsmmodem"
	"github.com/yazver/gsmmodem/sms"
	"time"
	"strings"
	"github.com/atotto/clipboard"
	"github.com/fatih/color"
	"github.com/xlab/at"
)

type Config struct {
	Port       string `json:"port"`
	NotifyPort string `json:"notify_port"`
	//Baud     int      `json:"baud"`
	Messages []string `json:"messages"`
}

func GetAppDir() (string, error) {
	return filepath.Abs(filepath.Dir(os.Args[0]))
}

func normalizePhoneNumber(number string) (normNumber string, err error) {
	normNumber = ""
	for _, c := range number {
		if c >= '0' && c <= '9' {
			normNumber += string(c)
		}
	}
	if len(normNumber) == 11 && normNumber[0] == '8' {
		normNumber = string('7') + normNumber[1:len(normNumber)]
	} else if len(normNumber) == 10 && normNumber[0] == '9' {
		normNumber = "7" + normNumber
	}
	if !(len(normNumber) == 11 && normNumber[0:2] == "79") {
		err = errors.New(fmt.Sprintf("Неверный формат номера : %s", number))
	} else {
		err = nil
	}
	return
}

func sendSMS(dev *gsmmodem.Device, message string, phoneNumber string) {
	defer color.Unset()
	log.Printf("Отправка сообщения на номер: %s\n", phoneNumber)
	err := dev.SendLongSMS(message, sms.PhoneNumber(phoneNumber))
	if err != nil {
		color.Set(color.FgHiRed)
		log.Printf("Ошибка отправки сообщения: %s\n", err.Error())
	} else {
		color.Set(color.FgHiGreen)
		log.Printf("Сообщение отправлено на номер: " + phoneNumber)
	}
}

func isExitCommand(str string) bool {
	str = strings.TrimSpace(strings.ToLower(str))
	return str == "exit" || str == "quit" || str == "q" || str == "выход"
}

func consoleInputNumber(dev *gsmmodem.Device, message string) {
	defer color.Unset()
	fmt.Print("Введите номер для отправки смс: ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := scanner.Text()
		if isExitCommand(text) {
			break
		}

		phoneNumber, err := normalizePhoneNumber(text)
		if err != nil {
			color.Set(color.FgHiRed)
			fmt.Println(err)
			color.Unset()
			fmt.Print("Введите номер заново: ")
			continue
		}

		sendSMS(dev, message, phoneNumber)
		fmt.Print("Введите номер для отправки смс: ")
	}
}

func clipboardInputNumber(dev *at.Device, message string) {
	ticker := time.NewTicker(time.Millisecond * 100)
	go func() {
		previousPhoneNumber := ""
		for range ticker.C {
			if phoneNumber, err := clipboard.ReadAll(); err == nil {
				if phoneNumber, err = normalizePhoneNumber(phoneNumber); err == nil {
					if phoneNumber != previousPhoneNumber {
						sendSMS(dev, message, phoneNumber)
						previousPhoneNumber = phoneNumber
					}
				}
			}
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Наберите \"выход\": ")
	for scanner.Scan() {
		text := scanner.Text()
		if isExitCommand(text) {
			break
		}
		fmt.Print("Наберите \"выход\" для прекращения работы.")
	}
	ticker.Stop()
}

func main() {
	appDir, err := GetAppDir()
	if err != nil {
		log.Println("Ошибка получения пути расположения приложения: ", err.Error())
		appDir = ""
	}

	logFilePath := filepath.Join(appDir, "log.txt")
	if logFile, e := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE, 0666); e == nil {
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	} else {
		log.Println("Невозсожно открыть лог файл: " + logFilePath)
	}

	config := &Config{}
	configFilePath := filepath.Join(appDir, "config.json")
	file, err := os.Open(configFilePath)
	if err != nil {
		log.Printf("Невозможно открыть файл (%s): (%s) \n", configFilePath, err.Error())
	}
	if err := json.NewDecoder(file).Decode(config); err != nil {
		log.Printf("Невозможно прочитать конфигурацию из файла (%s): (%s) \n", configFilePath, err.Error())
	}

	//m := modem.New(config.Port, config.Baud, "")
	//if err := m.Connect(); err != nil {
	//	log.Println("Ошибка подключения к модему: ", err)
	//} else {
	//	fmt.Println(m.SendSMS("79135535377", config.Messages[0]))
	//}

	// Connect to modem and initialize
	dev := &gsmmodem.Device{
		CommandPort: config.Port,
		NotifyPort:  config.NotifyPort,
	}
	if err = dev.Open(); err != nil {
		log.Println("Ошибка открытия порта: ", err)
		return
	}
	if err = dev.Init(gsmmodem.DeviceE173()); err != nil {
		log.Println("Ошибка инициализации порта: ", err)
		return
	}
	defer dev.Close()

	fmt.Println("Какое сообщение необходимо отправлять?")
	for index, value := range config.Messages {
		fmt.Printf("%d) %s\n", index+1, value)
	}
	fmt.Print("Введите номер необходимого сообщения и нажмите Enter: ")
	index := 0
	for {
		n, e := fmt.Scanf("%d", &index)
		if e == nil && n == 1 && index >= 1 && index <= len(config.Messages) {
			break
		}
		fmt.Print("Введен неверный номер повторите попытку:")
	}
	index = index - 1
	message := config.Messages[index]
	fmt.Println("Будет рассылаться следующее сообщение: ", message)

	fmt.Println("Режим работы: ")
	fmt.Println("1) Вводить вручную номер;")
	fmt.Println("2) Следить за буфером обмена.")
	fmt.Print("Введите режим работы и нажмите Enter: ")
	index = 0
	for {
		fmt.Scanf("%d", &index)
		if index == 1 {
			consoleInputNumber(dev, message)
			break
		} else if index == 2 {
			clipboardInputNumber(dev, message)
			break
		}
	}


}
