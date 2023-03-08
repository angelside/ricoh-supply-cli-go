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

	//ipAddr := "172.18.175.7"
	ipAddr := os.Args[1]

	valid_ip_address := validateIpAddress(ipAddr)
	if !valid_ip_address {
		fmt.Println("[ERROR] IP address is invalid!")
		os.Exit(0)
	}

	//
	// Data
	//

	// Alternative
	//getStatus(ipAddr)
	//supplyMap := makeSupplyMap()
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
func validateIpAddress(ipAddress string) bool {
	return net.ParseIP(ipAddress) != nil
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
	gosnmp.Default.Timeout = time.Duration(10 * time.Second) // Timeout better suited to walking
	err := gosnmp.Default.Connect()
	if err != nil {
		fmt.Printf("Connect err: %v\n", err)
		return err
	}
	return nil
}

// Depends: snmpConnection()
func getStatus(ipAddr string) {
	err := snmpConnection(ipAddr)
	if err != nil {
		fmt.Printf("Connect err: %v\n", err)
		os.Exit(1)
	}

	defer gosnmp.Default.Conn.Close()

	// Serial number / CxxxPxxxxxx
	serialNumber, err = func() (string, error) {
		data, err := gosnmp.Default.Get([]string{oid["serialNum"]})
		if err != nil {
			return "", err
		}
		return string(data.Variables[0].Value.([]byte)), nil
	}()

	// Model name / MP C307
	modelName, err = func() (string, error) {
		data, err := gosnmp.Default.Get([]string{oid["modelName"]})
		if err != nil {
			return "", err
		}
		return string(data.Variables[0].Value.([]byte)), nil
	}()

	// Supply names
	err = gosnmp.Default.BulkWalk(oid["supplyNames"], func(pdu gosnmp.SnmpPDU) error {
		supply_names = append(supply_names, string(pdu.Value.([]byte)))
		return nil
	})
	if err != nil {
		fmt.Printf("Walk Error: %v\n", err)
		os.Exit(1)
	}

	// Supply levels
	err = gosnmp.Default.BulkWalk(oid["supplyLevels"], func(pdu gosnmp.SnmpPDU) error {
		supply_levels = append(supply_levels, pdu.Value.(int))
		return nil
	})
	if err != nil {
		fmt.Printf("Walk Error: %v\n", err)
		os.Exit(1)
	}
}
