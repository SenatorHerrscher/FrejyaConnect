package main

import (
	"Frejya_Connect/Network"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	// 1. JSON bayrağını tanımla:
	jsonMode := flag.Bool("json", false, "Rust TUI için JSON çıktısı üret")
	flag.Parse()

	netInfo, err := Network.FinderIpAddr()
	if err != nil {
		fmt.Printf("Hata: %v\n", err)
		os.Exit(1)
	}

	devices, err := Network.ScannerDevices(netInfo)
	if err != nil {
		fmt.Printf("Hata: %v\n", err)
		os.Exit(1)
	}

	// 2. Eğer --json denildiyse sadece JSON bas ve çık (Rust burayı okuyacak):
	if *jsonMode {
		jsonData, _ := json.MarshalIndent(devices, "", "  ")
		fmt.Println(string(jsonData))
		return
	}

	// 3. Normal insan gözü için konsol çıktısı (Mevcut kodun):
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("✅ Aktif Kartımız : %s\n", netInfo.InterfaceName)
	fmt.Printf("📍 Bizim IP       : %s\n", netInfo.LocalIP)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for i, dev := range devices {
		fmt.Printf("  [%d] IP: %-15s | Tip: %-28s | Üretici: %s\n", i+1, dev.Ip, dev.DeviceType, dev.Vendor)
		for _, port := range dev.OpenPorts {
			fmt.Printf("       └── Açık Port: %-5d ➜ %s\n", port, Network.KnownPortServices[port])
		}
	}
}
