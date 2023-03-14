package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	gosnmp "github.com/gosnmp/gosnmp"
)

var oid = map[string]string{
	"modelName":    "1.3.6.1.2.1.43.5.1.1.16.1",
	"serialNum":    "1.3.6.1.2.1.43.5.1.1.17.1",
	"supplyNames":  "1.3.6.1.2.1.43.12.1.1.4.1",
	"supplyLevels": "1.3.6.1.2.1.43.11.1.1.9.1",
}

var supply_names = make([]string, 0, 4)
var supply_levels = make([]int, 0, 4)

var modelName = "N/A"
var serialNumber = "N/A"

func main() {
	//
	// Args
	//

	filename := filepath.Base(os.Args[0])

	if len(os.Args) != 2 {
		fmt.Println("Usage:", filename, "IpAddress")
		return
	}

	ipAddr := os.Args[1]
	if err := validateIpAddress(ipAddr); err != nil {
		fmt.Println(err)
		//os.Exit(1)
		return
	}

	//
	// Data
	//

	// FIXME: Need error handling
	supplyMap := func() map[string]int {
		getStatus(ipAddr)
		return makeSupplyMap()
	}()

	fmt.Println("")
	fmt.Printf("ip: %s - model: %s - serial: %s \n\n", ipAddr, modelName, serialNumber)

	for name, value := range supplyMap {
		fmt.Println(progressBar(name, value))
	}
}

// Validate ip address
func validateIpAddress(ipAddress string) error {
	if net.ParseIP(ipAddress) == nil {
		return errors.New("[ERROR] IP address is invalid!")
	}
	return nil
}

// Draw progress bar
func progressBar(text string, count int) string {
	barLen := 40
	total := 100
	emptyFill := "-"
	fill := "="

	percents := ""

	// -2 unknown toner
	if count < 0 {
		count = 0
		percents = "N/A"
		text = fmt.Sprintf("%s (Unknown toner)", text)
	} else {
		percents = fmt.Sprintf("%d%%", int64(100*count)/int64(total))
	}

	filledLen := int(float64(barLen) * float64(count) / float64(total))
	bar := strings.Repeat(string(fill), filledLen) + strings.Repeat(string(emptyFill), barLen-filledLen)

	return fmt.Sprintf("[%s] %s %s\r", bar, percents, text)
}

func makeSupplyMap() map[string]int {
	supplyMap := make(map[string]int)

	for i := 0; i < len(supply_names); i++ {
		supplyMap[supply_names[i]] = supply_levels[i]
	}

	// Delete waste toner
	delete(supplyMap, "other")

	// map[black:10 cyan:30 magenta:40 other:100 yellow:20]
	return supplyMap
}

func snmpConnection(ipAddr string) error {
	gosnmp.Default.Target = ipAddr
	gosnmp.Default.Community = "public"
	gosnmp.Default.Timeout = time.Duration(5 * time.Second) // Timeout better suited to walking

	if err := gosnmp.Default.Connect(); err != nil {
		return err
	}

	return nil
}

/*
192.168.0.1
Walk Error: request timeout (after 3 retries)
FIXME: Where is the retry settings ?
*/

// Depends: snmpConnection()
func getStatus(ipAddr string) error {
	var err error

	if err = snmpConnection(ipAddr); err != nil {
		fmt.Printf("[ERROR] Connection: %v\n", err)
		//os.Exit(1)
		return err
	}

	defer gosnmp.Default.Conn.Close()

	// Serial number / CxxxPxxxxxx
	if err := func() error {
		data, err := gosnmp.Default.Get([]string{oid["serialNum"]})
		if err != nil {
			return err
		}
		serialNumber = string(data.Variables[0].Value.([]byte))
		return nil
	}(); err != nil {
		fmt.Printf("[ERROR] Unable to retrieve 'serial number': %v\n", err)
		//os.Exit(1)
		return err
	}

	// Model name
	if err = func() error {
		data, err := gosnmp.Default.Get([]string{oid["modelName"]})
		if err != nil {
			return err
		}
		modelName = string(data.Variables[0].Value.([]byte))
		return nil
	}(); err != nil {
		fmt.Printf("[ERROR] Unable to retrieve 'model name': %v\n", err)
		//os.Exit(1)
		return err
	}

	// Supply names
	if err = gosnmp.Default.BulkWalk(oid["supplyNames"], func(pdu gosnmp.SnmpPDU) error {
		supply_names = append(supply_names, string(pdu.Value.([]byte)))
		return nil
	}); err != nil {
		fmt.Printf("[ERROR] Unable to retrieve 'supply names': %v\n", err)
		//os.Exit(1)
		return err
	}

	// Supply levels
	if err = gosnmp.Default.BulkWalk(oid["supplyLevels"], func(pdu gosnmp.SnmpPDU) error {
		supply_levels = append(supply_levels, pdu.Value.(int))
		return nil
	}); err != nil {
		fmt.Printf("[ERROR] Unable to retrieve 'supply levels': %v\n", err)
		//os.Exit(1)
		return err
	}

	return nil
}
