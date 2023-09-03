package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	gosnmp "github.com/gosnmp/gosnmp"
)

// SNMP settings
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

// Error messages, errorMsg["serialNum"]
var errorMsg = map[string]string{
	// General / fmt.Errorf
	"cliUsage":  "Usage: %s IpAddress",
	"invalidIP": "[ERROR] Invalid IP address: %s",
	// SNMP / fmt.Errorf
	"connection":   "[ERROR] Connection: %v\n",
	"serialNum":    "[ERROR] Unable to retrieve 'serial number': %v\n",
	"modelName":    "[ERROR] Unable to retrieve 'model name': %v\n",
	"supplyNames":  "[ERROR] Unable to retrieve 'supply names': %v\n",
	"supplyLevels": "[ERROR] Unable to retrieve 'supply levels': %v\n",
}

func main() {
	// Get ip address from argument
	ipAddress, err := getArgs()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Validate ip address
	if err := validateIpAddress(ipAddress); err != nil {
		fmt.Println(err)
		return
	}

	// Data
	data, err := getData(ipAddress)
	if err != nil {
		fmt.Println(err)
	}

	// Extract sub-data and type assert
	extraData := data["extra"].(map[string]string)
	toners := data["toners"].(map[string]int)

	fmt.Println("")
	fmt.Printf("ip: %s - model: %s - serial: %s \n\n", ipAddress, extraData["modelName"], extraData["serialNumber"])

	for name, value := range toners {
		fmt.Println(progressBar(name, value))
	}

}

// Get ip address from argument
func getArgs() (string, error) {
	// Get filename and exit if there is no argument
	filename := filepath.Base(os.Args[0])
	if len(os.Args) != 2 {
		return "", fmt.Errorf(errorMsg["cliUsage"], filename)
	}

	// Get ip address from argument
	ipAddress := os.Args[1]

	return ipAddress, nil
}

//
// SNMP
//

func snmpConnection(ipAddress string) error {
	gosnmp.Default.Target = ipAddress
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
func getData(ipAddress string) (map[string]interface{}, error) {
	// Output variables
	var supplyNames = make([]string, 0, 4)
	var supplyLevels = make([]int, 0, 4)
	var modelName = "N/A"
	var serialNumber = "N/A"

	if err := snmpConnection(ipAddress); err != nil {
		return nil, fmt.Errorf(errorMsg["connection"], err)
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
		return nil, fmt.Errorf(errorMsg["serialNum"], err)
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
		return nil, fmt.Errorf(errorMsg["modelName"], err)
	}

	// Supply names
	if err := gosnmp.Default.BulkWalk(oid["supplyNames"], func(pdu gosnmp.SnmpPDU) error {
		supplyNames = append(supplyNames, string(pdu.Value.([]byte)))
		return nil
	}); err != nil {
		return nil, fmt.Errorf(errorMsg["supplyNames"], err)
	}

	// Supply levels
	if err := gosnmp.Default.BulkWalk(oid["supplyLevels"], func(pdu gosnmp.SnmpPDU) error {
		supplyLevels = append(supplyLevels, pdu.Value.(int))
		return nil
	}); err != nil {
		return nil, fmt.Errorf(errorMsg["supplyLevels"], err)
	}

	// Merge supplyNames and supplyLevels and make a key=>value map
	supplyMap := makeSupplyMap(supplyNames, supplyLevels)

	// Prepare extra data
	extraData := map[string]string{
		"serialNumber": serialNumber,
		"modelName":    modelName,
	}

	// Prepare data
	data := map[string]interface{}{
		"toners": supplyMap,
		"extra":  extraData,
	}

	return data, nil
}

//
// Utils
//

// Merge supplyNames and supplyLevels and make a key=>value map
func makeSupplyMap(supplyNames []string, supplyLevels []int) map[string]int {
	supplyMap := make(map[string]int)

	for i := 0; i < len(supplyNames); i++ {
		supplyMap[supplyNames[i]] = supplyLevels[i]
	}

	// Delete waste toner (It always showed me 100)
	delete(supplyMap, "other")

	return supplyMap
}

// Validate ip address
func validateIpAddress(ipAddress string) error {
	if net.ParseIP(ipAddress) == nil {
		return fmt.Errorf(errorMsg["invalidIP"], ipAddress)
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

	// Make -2 to an Unknown toner
	// The printer showed -2% for non-genuine or recycled toner
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
