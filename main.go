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

var snmp = map[string]interface{}{
	"retry":   1,
	"timeout": 5 * time.Second,
}

// Input variables
var oid = map[string]string{
	"modelName":    "1.3.6.1.2.1.43.5.1.1.16.1",
	"serialNum":    "1.3.6.1.2.1.43.5.1.1.17.1",
	"supplyNames":  "1.3.6.1.2.1.43.12.1.1.4.1",
	"supplyLevels": "1.3.6.1.2.1.43.11.1.1.9.1",
}

// Output variables
var supplyNames = make([]string, 0, 4)
var supplyLevels = make([]int, 0, 4)
var modelName = "N/A"
var serialNumber = "N/A"

func main() {
	//
	// Args
	//

	// Get ip address from argument and valitade ip adress
	ipAddr, err := getArgs()
	if err != nil {
		fmt.Println(err)
		return
	}

	//
	// Data
	//

	if err := getData(ipAddr); err != nil {
		fmt.Println(err)
		return
	}

	// Merge supplyNames and supplyLevels and make a key=>value map
	supplyMap := makeSupplyMap()

	fmt.Println("")
	fmt.Printf("ip: %s - model: %s - serial: %s \n\n", ipAddr, modelName, serialNumber)

	for name, value := range supplyMap {
		fmt.Println(progressBar(name, value))
	}
}

func getArgs() (string, error) {
	// Get filename and exit if there is no argument (ip address)
	filename := filepath.Base(os.Args[0])
	if len(os.Args) != 2 {
		return "", fmt.Errorf("Usage: %s IpAddress", filename)
	}

	// Get ip address from argument
	ipAddr := os.Args[1]
	if err := validateIpAddress(ipAddr); err != nil {
		return "", err
	}

	return ipAddr, nil
}

//
// SNMP
//

func snmpConnection(ipAddr string) error {
	gosnmp.Default.Target = ipAddr
	gosnmp.Default.Community = "public"
	gosnmp.Default.Retries = snmp["retry"].(int)
	gosnmp.Default.Timeout = time.Duration(snmp["timeout"].(time.Duration)) // Timeout better suited to walking

	if err := gosnmp.Default.Connect(); err != nil {
		return err
	}

	return nil
}

// Get serialNumber, modelName, supplyNames, supplyLevels
// Depends: snmpConnection()
func getData(ipAddr string) error {
	if err := snmpConnection(ipAddr); err != nil {
		return fmt.Errorf("[ERROR] Connection: %v\n", err)
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
		return fmt.Errorf("[ERROR] Unable to retrieve 'serial number': %v\n", err)
	}

	// Model name
	if err := func() error {
		data, err := gosnmp.Default.Get([]string{oid["modelName"]})
		if err != nil {
			return err
		}
		modelName = string(data.Variables[0].Value.([]byte))
		return nil
	}(); err != nil {
		return fmt.Errorf("[ERROR] Unable to retrieve 'model name': %v\n", err)
	}

	// Supply names
	if err := gosnmp.Default.BulkWalk(oid["supplyNames"], func(pdu gosnmp.SnmpPDU) error {
		supplyNames = append(supplyNames, string(pdu.Value.([]byte)))
		return nil
	}); err != nil {
		return fmt.Errorf("[ERROR] Unable to retrieve 'supply names': %v\n", err)
	}

	// Supply levels
	if err := gosnmp.Default.BulkWalk(oid["supplyLevels"], func(pdu gosnmp.SnmpPDU) error {
		supplyLevels = append(supplyLevels, pdu.Value.(int))
		return nil
	}); err != nil {
		return fmt.Errorf("[ERROR] Unable to retrieve 'supply levels': %v\n", err)
	}

	return nil
}

//
// Utils
//

// Merge supplyNames and supplyLevels and make a key=>value map
func makeSupplyMap() map[string]int {
	supplyMap := make(map[string]int)

	for i := 0; i < len(supplyNames); i++ {
		supplyMap[supplyNames[i]] = supplyLevels[i]
	}

	// Delete waste toner
	delete(supplyMap, "other")

	// map[black:10 cyan:30 magenta:40 other:100 yellow:20]
	return supplyMap
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
