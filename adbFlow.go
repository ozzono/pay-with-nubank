package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	comgas "github.com/ozzono/comgas_invoice"

	enel "github.com/ozzono/enel_invoice"
)

var (
	headless   bool
	configPath string
	hookURL    string
)

type invoice struct {
	Provider string
	DueDate  string
	Value    string
	BarCode  string
	Status   string
}

type Enel struct {
	User enel.UserData `json:"user"`
}

type Comgas struct {
	User comgas.UserData `json:"user"`
}

type config struct {
	Enel    Enel   `json:"enel"`
	Comgas  Comgas `json:"comgas"`
	HookURL string `json:"hookURL"`
}

func init() {
	flag.StringVar(&configPath, "config", "config.json", "Sets the path to user data in JSON format")
	flag.BoolVar(&headless, "headless", false, "Enables or disables headless chrome navigation")
}

func main() {
	flag.Parse()
	invoices := []invoice{}
	config, err := setConfig()
	if err != nil {
		log.Println(err)
		return
	}
	invoice, err := comgasInvoice(config.Comgas.User)
	if err != nil {
		log.Printf("comgasInvoice err: %v", err)
		return
	}
	invoices = append(invoices, invoice)

	invoice, err = enelInvoice(config.Enel.User)
	if err != nil {
		log.Printf("comgasInvoice err: %v", err)
		return
	}
	invoices = append(invoices, invoice)

	log.Printf("%#v", invoices)
}

func enelInvoice(user enel.UserData) (invoice, error) {
	f := enel.NewFlow(headless)
	f.User = user
	invoiceData, err := f.InvoiceFlow()
	if err != nil {
		log.Printf("f.InvoiceFlow err: %v", err)
		return invoice{}, err
	}
	return invoice{
		Provider: "enel",
		DueDate:  invoiceData.DueDate,
		Value:    invoiceData.Value,
		BarCode:  invoiceData.BarCode,
		Status:   invoiceData.Status,
	}, nil
}

func comgasInvoice(user comgas.UserData) (invoice, error) {
	f := comgas.NewFlow(headless)
	f.User = user
	invoiceData, err := f.InvoiceFlow()
	if err != nil {
		log.Printf("f.InvoiceFlow err: %v", err)
		return invoice{}, err
	}
	return invoice{
		Provider: "comgas",
		DueDate:  invoiceData.DueDate,
		Value:    invoiceData.Value,
		BarCode:  invoiceData.BarCode,
		Status:   invoiceData.Status,
	}, nil
}

func setConfig() (config, error) {
	if len(configPath) == 0 {
		return config{}, fmt.Errorf("invalid path; cannot be empty")
	}
	jsonFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config{}, err
	}
	config := config{}
	err = json.Unmarshal(jsonFile, &config)
	return config, err
}

func (invoice *invoice) toText() string {
	return fmt.Sprintf(
		"%s successfully captured:\nDueDate: %s\nValue: %s\nStatus: %s",
		invoice.Provider,
		invoice.DueDate,
		invoice.Value,
		invoice.Status,
	)
}

func slackMsg(body, hookURL string) error {
	log.Println("Sending slack message")
	payload := strings.NewReader(fmt.Sprintf("{\"text\":\"%s\"}", body))

	req, err := http.NewRequest("POST", hookURL, payload)
	if err != nil {
		return err
	}

	req.Header.Add("content-type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("response statusCode: %v\nres: %v", res.StatusCode, res)
	}
	return nil
}
