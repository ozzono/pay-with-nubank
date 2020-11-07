package nubank

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	adb "github.com/ozzono/adbtools"
)

var (
	nubank = app{
		pkg:      "com.nu.production",
		activity: "br.com.nubank.shell.screens.splash.SplashActivity",
	}
)

type app struct {
	pkg      string
	activity string
}

// Flow contains all the needed data to pay the invoices
type Flow struct {
	device  adb.Device
	Invoice Invoice
}

// Invoice has all the payment needed data
type Invoice struct {
	DueDate string
	Value   string
	BarCode string
	Status  string
}

// NewFlow creates the needed flow allowing the payment
func NewFlow(invoice Invoice) (Flow, error) {
	devices, err := adb.Devices(true)
	if err != nil {
		return Flow{}, err
	}
	for i := range devices {
		if !strings.Contains(devices[i].ID, "emulator") {
			return Flow{
				device:  devices[i],
				Invoice: invoice,
			}, nil
		}
	}
	return Flow{}, fmt.Errorf("failed to created new flow")
}

// PayInvoice flows through the app screens to scheduled a payment
func (flow *Flow) PayInvoice() error {
	log.Println("starting nubank payment flow")
	if err := flow.device.WaitDeviceReady(5); err != nil {
		return err
	}
	oldTimer, err := flow.device.ScreenTimeout("10m")
	if err != nil {
		oldTimer()
		return err
	}
	defer oldTimer()

	if err := flow.device.ScreenSize(); err != nil {
		return err
	}
	flow.device.WakeUp()
	flow.device.Swipe(
		[4]int{
			flow.device.Screen.Width / 2,
			flow.device.Screen.Height - 100,
			flow.device.Screen.Width / 2,
			100,
		},
	)
	flow.device.CloseApp(nubank.pkg)
	if err := flow.device.StartApp(nubank.pkg, nubank.activity, ""); err != nil {
		return err
	}
	if !flow.device.WaitApp(nubank.pkg, 10, 5) {
		return fmt.Errorf("failed to start %s", nubank.pkg)
	}

	for err := flow.device.WaitInScreen(len(expList())/3, "Pagar"); err != nil; {
		for _, item := range expList() {
			screen, err := flow.device.XMLScreen(true)
			if err != nil {
				return err
			}
			if !match(item, screen) {
				continue
			}
			coords, err := adb.XMLtoCoords(item)
			if err != nil {
				return err
			}
			flow.device.Swipe([4]int{coords[0], coords[1], 100, coords[1]})
			break
		}
		return fmt.Errorf("unable to find invoice payment button")
	}

	err = flow.device.Exp2Tap(expList()["pagar-btn"])
	if err != nil {
		return err
	}

	err = flow.device.WaitInScreen(5, "Pagar um boleto")
	if err != nil {
		return err
	}

	err = flow.device.Exp2Tap(expList()["pay-invoice"])
	if err != nil {
		return err
	}

	err = flow.device.WaitInScreen(5, "inserir codigo")
	if err != nil {
		return err
	}

	err = flow.device.Exp2Tap(expList()["insert-code"])
	if err != nil {
		return err
	}
	time.Sleep(time.Duration(flow.device.DefaultSleep*30) * time.Millisecond)

	barcode := []string{}
	for _, item := range strings.Split(flow.Invoice.BarCode, ".") {
		for _, innerItem := range strings.Split(item, " ") {
			barcode = append(barcode, innerItem)
		}
	}
	err = flow.device.InputText(flow.Invoice.BarCode, true)
	if err != nil {
		return err
	}

	err = flow.device.Exp2Tap(expList()["continue-btn"])
	if err != nil {
		return err
	}

	err = flow.device.WaitInScreen(5, "Este é o próximo")
	if err != nil {
		log.Printf("the text 'Este é o próximo' did not appear on screen")
	} else {
		time.Sleep(1 * time.Second)
		flow.device.TapScreen(flow.device.Screen.Width/2, flow.device.Screen.Height/2, 5)
	}

	time.Sleep(1 * time.Second)
	if !flow.device.HasInScreen(true, flow.Invoice.Value) {
		return fmt.Errorf("invoice value not found")
	}

	time.Sleep(1 * time.Second)
	invoiceDay := strings.Split(flow.Invoice.DueDate, "/")[0]
	todayDay := fmt.Sprintf("%d", int(time.Now().Day()))
	if invoiceDay != todayDay {
		err = flow.dateInput(flow.Invoice.DueDate)
		if err != nil {
			return err
		}
	}

	return nil
}
func (flow *Flow) dateInput(date string) error {
	log.Println("starting date input flow")
	invoiceMonth := strings.Split(date, "/")[1]
	todayMonth := fmt.Sprintf("%d", int(time.Now().Month()))
	err := flow.device.Exp2Tap(expList()["date-btn"])
	if err != nil {
		return err
	}

	err = flow.device.WaitInScreen(10, "Para qual dia útil você quer agendar")
	if err != nil {
		return err
	}
	if invoiceMonth > todayMonth {
		log.Println("swiping to next month")
		screen, err := flow.device.XMLScreen(true)
		if err != nil {
			return err
		}
		xmlCoords := applyRegexp(expList()["date-continue"], screen)[1]
		if !match("(\\[\\d+,\\d+)", xmlCoords) {
			return fmt.Errorf("failed to fetch 'continue' button coords")
		}
		maxHeight, err := strconv.Atoi(strings.Split(string(xmlCoords[1:]), ",")[1])
		if err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
		flow.device.Swipe([4]int{
			flow.device.Screen.Width / 2,
			maxHeight - int(float64(flow.device.Screen.Height)*0.08),
			flow.device.Screen.Width / 2,
			maxHeight - int(float64(flow.device.Screen.Height)*0.37), // scrools throught nearly 30% of the screen size
		})
	}

	err = flow.device.Exp2Tap(
		fmt.Sprintf(
			expList()["day"],
			strings.Split(flow.Invoice.DueDate, "/")[0],
			strings.Split(flow.Invoice.DueDate, "/")[0],
		))
	if err != nil {
		return err
	}

	err = flow.device.Exp2Tap(expList()["continue-btn"])
	if err != nil {
		return err
	}

	return nil
}

func applyRegexp(exp, text string) []string {
	re := regexp.MustCompile(exp)
	matches := re.FindStringSubmatch(text)
	if len(matches) < 1 {
		log.Printf("unable to find match for exp %s\n", exp)
		return []string{}
	}
	return matches
}

func expList() map[string]string {
	// (\[\d+,\d+\]\[\d+,\d+\])
	return map[string]string{
		// list of all buttons available in the footer menu row
		"pix":            "Pix.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"pagar-btn":      "Pagar.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"indicar-amigos": "Indicar.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"transferir":     "Transferir.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"depositar":      "Depositar.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"emprestimos":    "Empréstimos.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"cartao-virtual": "Cartão.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"recarga":        "Recarga.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"ajustar-limite": "Ajustar.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"bloquear":       "Bloquear.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"cobrar":         "Cobrar.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"docao":          "Doação.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"ajuda":          "ajuda.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",

		"pay-invoice":  "\"Contas de luz.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"insert-code":  "text=\"INSERIR CÓDIGO.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"code-input":   "código do boleto.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"continue-btn": "text=\"CONTINUAR.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",

		"value":         "text=\"R..(\\d+,\\d{2})",
		"date-btn":      "\\d{2}/\\d{2}/20\\d{2}.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
		"date-continue": "CONTINUAR.*?(\\[\\d+,\\d+)\\]",
		"day":           "text=\"%s.*text=\"%s.*?(\\[\\d+,\\d+\\]\\[\\d+,\\d+\\])",
	}
}

func match(exp, text string) bool {
	return regexp.MustCompile(exp).MatchString(text)
}

func waitEnter() {
	log.Printf("Press <enter> to continue or <ctrl+c> to interrupt")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	log.Printf("Now, where was I?")
	log.Printf("Oh yes...\n\n")
}
