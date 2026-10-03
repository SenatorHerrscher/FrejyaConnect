package Network

import (
	"Frejya_Connect/DTO"
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func FinderIpAddr() (*DTO.NetworkInfo, error) {

	interfaces, error := net.Interfaces() //NIC kart
	if error != nil {
		return nil, fmt.Errorf("NIC karti bulunamadi{orn:etn1,wlan0}:%v", error)
	}

	for _, interfaAttribute := range interfaces {
		if interfaAttribute.Flags&net.FlagLoopback != 0 || interfaAttribute.Flags&net.FlagUp == 0 { //localhost mu  kart yazilimsal olarak acik mi
			continue
		}
		name := strings.ToLower(interfaAttribute.Name)
		if strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "tailscale") {
			continue
		}

		addr, err := interfaAttribute.Addrs()
		if err != nil {
			return nil, fmt.Errorf("Addr Bulunumadi{Ip addr bulunamadi....}:%v", err) // icinde ipv4 ve ipv6 olan MAc tamamen farkli
		}

		for _, addr := range addr {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ipv4 := ipnet.IP.To4()
			if ipv4 == nil {
				continue
			}

			baseIp := ipv4.Mask(ipnet.Mask) //192.168.1.0

			ones, bits := ipnet.Mask.Size()       // ff-ff-ff-00(255.255.255.0) ipv4 32 byte  ve /24 deki 24 sayisi
			totalHost := (1 << (bits - ones)) - 2 // bits ones cikar 2 uzerinde al ve broadcast ve agin kendisi(192.168.1.0) oldugu icin de -2

			return &DTO.NetworkInfo{
				InterfaceName: interfaAttribute.Name,
				LocalIP:       ipv4,
				Subnet: &net.IPNet{
					IP:   baseIp,
					Mask: ipnet.Mask,
				},
				TotalHosts: totalHost,
			}, nil
		}
	}

	/*
		type Interface struct {
		Index        int          // Sistem tarafından atanan benzersiz arayüz indeksi
		MTU          int          // Maksimum İletim Birimi (Maximum Transmission Unit)
		Name         string       // Arayüzün adı (Örn: "eth0", "wlan0", "en0")
		HardwareAddr HardwareAddr // Donanım (MAC) adresi (Örn: 00:1a:2b:3c:4d:5e)
		Flags        Flags        // Arayüzün durumu (Örn: Çalışıyor mu, Up/Down, Multicast destekliyor mu)
			}

	*/

	return nil, error
}

func ScannerDevices(IpAddr *DTO.NetworkInfo) ([]DTO.DeviceOutput, error) {

	baseip := IpAddr.Subnet.IP.To4() //192.168.1.0

	var wg sync.WaitGroup //gorutline icin

	for i := 1; i <= IpAddr.TotalHosts; i++ {
		targetip := make(net.IP, len(baseip)) //gorutline icin ikiz liste
		copy(targetip, baseip)                // [] cop etme race condition ve veri guvenligi icin Gorutlinelar
		targetip[3] = byte(i)

		wg.Add(1)
		go func(ip net.IP) {
			defer wg.Done()
			pingUDP(ip)
		}(targetip)
	}

	wg.Wait()
	time.Sleep(150 * time.Millisecond)
	file, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, fmt.Errorf("Arp Tablosu Okunamadi....:%v", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Scan()

	var devices []DTO.DeviceOutput

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 6 { // [ip(0)][HWR(1)][FLAGS(2)][MAC(3)][MASK(4)][Device(5)]
			continue
		}

		ipStr := fields[0]
		flags := fields[2]
		macStr := fields[3]
		devName := fields[5]

		if devName != IpAddr.InterfaceName || flags != "0x2" { //0x2 MAC verebilir//
			continue
		}

		// MAC adresini Go tipine çeviriyoruz
		mac, err := net.ParseMAC(macStr)
		if err != nil {
			continue
		}

		deviceIP := net.ParseIP(ipStr)
		openPorts := chechPorts(deviceIP)
		vendor := GetVender(mac)
		deviceTypes := DetectDeviceType(openPorts, vendor)
		devices = append(devices, DTO.DeviceOutput{
			Ip:         net.ParseIP(ipStr),
			Mac:        mac,
			Vendor:     vendor,
			OpenPorts:  openPorts,
			DeviceType: deviceTypes,
		})
	}

	// Fonksiyonun sonunda bulunan cihazları dönüyoruz!
	return devices, nil
}
func pingUDP(ip net.IP) {

	devices := net.JoinHostPort(ip.String(), "12345") //192.168.1.[i]:12345
	conn, err := net.DialTimeout("udp", devices, 50*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close() //↓
	//↓
	// defer close func sonu calisir
	conn.Write([]byte{0})
}

func GetVender(mac net.HardwareAddr) string {

	if mac[0]&0x02 != 0 {
		return "📱 Gizli / Rastgele MAC (Android/iOS)"
	}
	if len(mac) < 3 {
		return "bilinmeyen"
	}
	oui := fmt.Sprintf("%02X:%02X:%02X", mac[0], mac[1], mac[2]) // Çıktısı: "C0:49:43"

	vendors := map[string]string{
		// Senin Evindeki Canlı Cihazlar:
		"C0:49:43": "🌐 ZTE Corporation (Fiber Modem)",
		"00:1E:73": "🌐 ZTE Corporation (Modem / Gateway)",
		"48:46:FB": "🌐 Huawei Technologies (Fiber ONT / Router)",
		"00:E0:FC": "🌐 Huawei Technologies (Modem)",
		"7C:03:D8": "🌐 Sagemcom Broadband (Türk Telekom / Vodafone)",
		"18:83:BF": "🌐 Sagemcom Broadband (Fiber Gateway)",
		"50:67:F0": "🌐 Zyxel Communications (VDSL / Fiber)",
		"AC:CF:85": "🌐 Zyxel Communications (Modem)",
		"50:C7:BF": "🌐 TP-Link Technologies (Wi-Fi Router)",
		"14:CC:20": "🌐 TP-Link Technologies (Router)",
		"50:FF:20": "🌐 Keenetic (Gelişmiş Router)",
		"F0:9F:C2": "🌐 Ubiquiti Inc (UniFi Access Point)",
		"64:D1:54": "🌐 MikroTik (RouterBOARD)",
		"00:18:E7": "🌐 Netgear Networks",
		"00:08:54": "🌐 Netgear (Nighthawk Router)",
		"34:31:C4": "🌐 AVM GmbH (FRITZ!Box - Avrupa Modemi)",
		// 📺 TELEVİZYONLAR & MEDYA OYNATICILAR
		"0C:CA:FB": "📺 Philips Smart TV (TP Vision Europe)",
		"70:AF:24": "📺 Philips Smart TV (Android 14)",
		"00:1A:79": "📺 LG Electronics (webOS 4K TV)",
		"A8:23:FE": "📺 LG Electronics (Smart Device)",
		"70:85:C2": "📺 Sony Bravia 4K TV",
		"F8:46:1C": "📺 Sony Bravia Smart TV",
		"00:1E:E3": "📺 Vestel Elektronik (Smart TV)",
		"18:28:61": "📺 Vestel Elektronik (Toshiba / Regal TV)",
		"D8:07:B6": "📺 Google LLC (Chromecast / Google TV)",
		"3C:5A:37": "📺 Google LLC (Nest / Chromecast)",
		"44:65:0D": "📺 Amazon (Fire TV Stick 4K)",
		"B4:7C:9C": "🔊 Amazon (Echo Dot / Alexa Hoparlör)",
		"00:0D:4B": "📺 Roku Inc (Roku Streaming Stick)",
		"58:82:A8": "🔊 Sonos Inc (Akıllı Ses Sistemi)",
		// 💡 AKILLI EV (IoT), SENSÖRLER & ROBOT SÜPÜRGELER
		"70:89:76": "💡 Tuya Smart (Akıllı Priz / Akıllı Ampul)",
		"10:D5:61": "💡 Tuya Smart Inc (IoT Röle)",
		"D4:A6:51": "💡 Tuya Smart (Akıllı Ev Cihazı)",
		"24:0A:C4": "💡 Espressif Inc (ESP32 Akıllı Çip / Sonoff)",
		"30:AE:A4": "💡 Espressif Inc (Akıllı Lamba / Röle)",
		"84:F3:EB": "💡 Espressif Inc (ESP8266 IoT Sensör)",
		"BC:10:2F": "💡 SJI Industry (Akıllı Priz / Akıllı Ev)",
		"88:B5:FF": "💡 Shenzhen iComm (IoT Akıllı Sensör)",
		"64:90:C1": "🧹 Roborock Technology (Robot Süpürge)",
		"34:CE:00": "🧹 Dreame Innovation (Robot Süpürge)",
		"00:0A:F5": "💡 Shelly / Allterco (Akıllı Ev Rölesi)",
		"EC:1B:BD": "📹 TP-Link Tapo (Güvenlik Kamerası)",
		"40:ED:98": "📹 Eufy Security (Anker Akıllı Kamera)",
		"54:E0:19": "💡 Signify Netherlands (Philips Hue Bridge)",
		"00:03:7F": "🧺 Arçelik / Beko (HomeWhiz Akıllı Beyaz Eşya)",
		// 🎮 OYUN KONSOLLARI
		"FC:F1:52": "🎮 Sony Interactive (PlayStation 5)",
		"00:13:15": "🎮 Sony Interactive (PlayStation 4)",
		"7C:1E:52": "🎮 Microsoft Corp (Xbox Series X / S)",
		"DC:98:40": "🎮 Microsoft Corp (Xbox One)",
		"98:B6:E9": "🎮 Nintendo Co., Ltd. (Nintendo Switch)",
		"70:48:0F": "🎮 Nintendo Co., Ltd. (Switch OLED)",
		"74:DE:2B": "🎮 Valve Corporation (Steam Deck)",
		// 💻 BİLGİSAYARLAR, ANAKARTLAR & İŞLEMCİLER
		"10:7C:61": "💻 ASUSTek Computer (Masaüstü Anakart)",
		"04:D4:C4": "💻 ASUSTek Computer (ROG / TUF Gaming)",
		"44:E5:17": "💻 Intel Corporation (PC Wi-Fi / NUC)",
		"80:86:F2": "💻 Intel Corporation (Gigabit NIC)",
		"EC:F4:BB": "💻 Dell Technologies (XPS / Inspiron Laptop)",
		"A4:4C:C8": "💻 Lenovo Group (ThinkPad / Legion)",
		"54:EE:75": "💻 HP Inc. (Omen / Victus / Laptop)",
		"00:D8:61": "💻 Micro-Star Int'l (MSI Anakart)",
		"E0:D5:5E": "💻 GIGA-BYTE Technology (Aorus Anakart)",
		"B8:27:EB": "🍓 Raspberry Pi Foundation (Pi 3)",
		"DC:A6:32": "🍓 Raspberry Pi Trading (Pi 4 / Pi 5)",
		"00:1C:42": "💻 Parallels (Sanal Makine)",
		"00:50:56": "💻 VMware (Sanal Makine / ESXi)",
		"08:00:27": "💻 Oracle (VirtualBox Sanal Makine)",
		// 📱 AKILLI TELEFONLAR & TABLETLER
		"F0:18:98": "🍏 Apple Inc. (iPhone / iPad / Mac)",
		"BC:D0:74": "🍏 Apple Inc. (MacBook / iDevice)",
		"AC:BC:32": "🍏 Apple Inc. (Apple Watch / iPad)",
		"68:9C:70": "🍏 Apple Inc. (iPhone Pro Max)",
		"30:07:4D": "📱 Samsung Electronics (Galaxy Cihaz)",
		"64:1C:AE": "📱 Samsung Electronics (Galaxy S / Z Fold)",
		"64:DD:E9": "📱 Xiaomi Communications (Redmi / POCO)",
		"50:EC:50": "📱 Xiaomi Mobile (Mi Serisi)",
		"78:11:DC": "📱 Xiaomi Communications (Tablet / Telefon)",
		"AC:E2:15": "📱 Huawei Device Co. (Telefon / MatePad)",
		"94:65:2D": "📱 OnePlus Technology",
		"2C:59:8A": "📱 OPPO Electronics",
		"54:60:09": "📱 Google LLC (Pixel Telefon)",
	}
	vendor, exists := vendors[oui]
	if !exists {
		return "Bilinmeyen Marka (" + oui + ")"
	}
	return vendor
}

// Paket seviyesinde tanımlıyoruz ki main.go ve TUI de hangi portun ne olduğunu görebilsin:
// Paket seviyesinde tanımlı Bilinen Port ve Servis Haritası:
var KnownPortServices = map[int]string{
	// ─── 📱 Android & Smart TV Sistemleri ───
	5555: "🎯 Android ADB (TV / Telefon - Scrcpy)",
	3000: "📺 LG webOS Smart TV (WebSocket API)",
	3001: "📺 LG webOS Smart TV (Güvenli SSL API)",
	8001: "📺 Samsung Tizen Smart TV (API v2 HTTP)",
	8002: "📺 Samsung Tizen Smart TV (API v2 SSL/WSS)",
	8008: "📺 Google Cast / Chromecast (HTTP)",
	8009: "📺 Google Cast / Chromecast (HTTPS)",
	7000: "📺 Apple AirPlay (Ekran Yansıtma)",
	7100: "📺 Apple AirPlay (Ses / Medya)",

	// ─── 🪟 Windows Sistemleri ───
	135:  "🪟 Windows RPC (EPMAP Servis Yöneticisi)",
	139:  "🪟 Windows NetBIOS (Dosya & Yazıcı Paylaşımı)",
	445:  "🪟 Windows SMB (Ağ Dosya Paylaşımı / Samba)",
	3389: "🪟 Windows RDP (Uzak Masaüstü Bağlantısı)",
	5985: "🪟 Windows WinRM (PowerShell Remote HTTP)",
	5986: "🪟 Windows WinRM (PowerShell Remote HTTPS)",

	// ─── 🐧 Linux & Unix Sistemleri ───
	22:    "🐧 Linux / SSH (Uzak Güvenli Terminal)",
	111:   "🐧 Linux RPCbind (NFS Portmapper)",
	2049:  "🐧 Linux NFS (Ağ Dosya Sistemi)",
	9090:  "🐧 Linux Cockpit (Web Sunucu Yönetim Paneli)",
	10000: "🐧 Linux Webmin (Sunucu Yönetim Paneli)",
	631:   "🖨️ CUPS Yazıcı Sunucusu (Linux / macOS)",

	// ─── 🍏 Apple Sistemleri (macOS / iOS) ───
	62078: "🍏 Apple iOS Sync (Lockdownd Wi-Fi Eşitleme)",
	548:   "🍏 Apple AFP (Apple Filing Protocol)",
	3689:  "🍏 Apple DAAP (iTunes / Müzik Paylaşımı)",

	// ─── 🎬 Medya Sunucuları & NAS ───
	32400: "🎬 Plex Medya Sunucusu",
	8096:  "🎬 Jellyfin Medya Sunucusu",
	5000:  "📦 Synology DSM / UPnP Medya Sunucusu",

	// ─── 🌐 Ağ, Modem & Ağ Geçidi Servisleri ───
	80:   "🌐 HTTP (Modem / Router / Web Paneli)",
	443:  "🔒 HTTPS (Güvenli Web Paneli / SSL)",
	53:   "📡 DNS Sunucusu (Gateway / Pi-hole / AdGuard)",
	21:   "📁 FTP (Dosya Transfer Sunucusu)",
	23:   "📟 Telnet (Router / Switch Konsolu)",
	8080: "⚙️ Alternatif HTTP (Web Proxy / Dev Sunucu)",
	8443: "⚙️ Alternatif HTTPS (Güvenli Web Portu)",

	// ─── 💡 IoT & Ev Otomasyonu ───
	554:  "📹 RTSP (IP Güvenlik Kamerası Canlı Video)",
	1883: "💡 MQTT Broker (Akıllı Ev Sensör İletişimi)",
	8123: "🏠 Home Assistant (Akıllı Ev Web Arayüzü)",
}

func chechPorts(ip net.IP) []int {
	var open []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Map'teki tüm portları eşzamanlı (paralel) tara:
	for port := range KnownPortServices {
		wg.Add(1)

		go func(p int) {
			defer wg.Done()

			target := net.JoinHostPort(ip.String(), strconv.Itoa(p))
			conn, err := net.DialTimeout("tcp", target, 80*time.Millisecond)

			if err == nil {
				conn.Close()
				// Birden fazla goroutine aynı anda listeye yazmasın diye kilitliyoruz (Race condition koruması):
				mu.Lock()
				open = append(open, p)
				mu.Unlock()
			}
		}(port)
	}

	// Tüm port taramalarının bitmesini bekle:
	wg.Wait()

	return open
}

func DetectDeviceType(ports []int, vendor string) string {
	hasPort := func(target int) bool {
		for _, p := range ports {
			if p == target {
				return true
			}
		}
		return false
	}

	if hasPort(5555) {
		return "📱 Android / Smart TV (ADB)"
	}
	if hasPort(22) {
		return "🐧 Linux Cihazı (SSH)"
	}
	if hasPort(3389) || hasPort(445) || hasPort(139) {
		return "🪟 Windows PC (RDP/SMB)"
	}
	if hasPort(8001) || hasPort(8002) {
		return "📺 Samsung Smart TV"
	}
	if hasPort(3000) || hasPort(3001) {
		return "📺 LG Smart TV (webOS)"
	}
	if hasPort(80) || hasPort(443) || hasPort(53) {
		return "🌐 Modem / Ağ Geçidi"
	}
	if hasPort(1883) || hasPort(8123) {
		return "💡 Akıllı Ev / IoT"
	}
	vLower := strings.ToLower(vendor)
	if strings.Contains(vLower, "apple") || strings.Contains(vLower, "iphone") {
		return "🍏 Apple Cihazı (iOS/macOS)"
	}
	if strings.Contains(vLower, "gizli") || strings.Contains(vLower, "rastgele") {
		return "📱 Mobil Cihaz (Gizli MAC)"
	}
	if strings.Contains(vLower, "tv") {
		return "📺 Smart TV"
	}
	return "🔌 Genel Ağ Cihazı"
}
