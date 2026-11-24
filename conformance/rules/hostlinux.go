package rules

import (
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Linux 上运行信息各项的取法（RFC-0023「平台读取路径」的「Linux 的取法」，ADR-0286）：输入是 /proc 与 /sys 里的文本，
// 读不到的项为空（nil 或假），不以 0 或空串顶替。

// cleanHostText 去掉首尾的空白与结尾的空字符；结果为空返回假（RFC-0023「契约」：字符串去掉首尾空白后为空即视为读不到）。
func cleanHostText(text string) (string, bool) {
	text = strings.TrimFunc(strings.TrimRight(strings.TrimFunc(text, IsWhitespace), "\x00"), IsWhitespace)
	return text, text != ""
}

// hostFirstLine 给出文本的第一行清理之后的内容；为空返回假。
func hostFirstLine(text string) (string, bool) {
	line, _, _ := strings.Cut(text, "\n")
	return cleanHostText(line)
}

// hostField 在逐行写成「名字 分隔符 值」的文本里取第一个名字相符的行的值：名字是分隔符之前去掉两侧空白的部分，区分大小写；
// 值清理之后为空返回假，不再往后找同名的行。
func hostField(text, key string, separator byte) (string, bool) {
	for line := range strings.SplitSeq(text, "\n") {
		index := strings.IndexByte(line, separator)
		if index > 0 && strings.TrimFunc(line[:index], IsWhitespace) == key {
			return cleanHostText(line[index+1:])
		}
	}
	return "", false
}

// isHostDigits 判断文本是否只由 ASCII 数字组成（至少一位）。
func isHostDigits(text string) bool {
	if text == "" {
		return false
	}
	for index := 0; index < len(text); index++ {
		if text[index] < '0' || text[index] > '9' {
			return false
		}
	}
	return true
}

// LinuxCPUTimes 由 /proc/stat 的内容读出系统 CPU 的累计时间：取第一个以 cpu 加空格起头的行，按空格分列，取名字之后的前八列
// （user、nice、system、idle、iowait、irq、softirq、steal；不足八列有几列算几列，至少四列）。空闲是 idle 加 iowait，
// 总计是这几列之和；之后的 guest 两列已经计入 user 与 nice，不再加。没有这一行、不足四列或这几列里有不是十进制数字的返回假。
func LinuxCPUTimes(stat string) (HostCPUTimes, bool) {
	for line := range strings.SplitSeq(stat, "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		columns := strings.FieldsFunc(line, func(char rune) bool { return char == ' ' })[1:]
		if len(columns) > 8 {
			columns = columns[:8]
		}
		if len(columns) < 4 {
			return HostCPUTimes{}, false
		}
		var times HostCPUTimes
		for index, column := range columns {
			value, err := strconv.ParseUint(column, 10, 64)
			if !isHostDigits(column) || err != nil {
				return HostCPUTimes{}, false
			}
			times.Total += value
			// 第四列是 idle，第五列是 iowait：等磁盘的时间里处理器没有在干活，算作空闲
			if index == 3 || index == 4 {
				times.Idle += value
			}
		}
		return times, true
	}
	return HostCPUTimes{}, false
}

// HostMemoryReading 是读到的物理内存（字节）：总量、可用、空闲与缓存，nil 表示读不到。
type HostMemoryReading struct {
	Total     *int64
	Available *int64
	Free      *int64
	Cached    *int64
}

// LinuxMemory 由 /proc/meminfo 的内容读出物理内存：MemTotal、MemAvailable、MemFree 各取同名的第一行；缓存是 Buffers 加 Cached，
// 只有一项时就是那一项，都没有为空。值是冒号之后到第一个空格之前的十进制数字，单位是 kB，乘以 1024；不是数字的那一项为空。
func LinuxMemory(meminfo string) HostMemoryReading {
	kilobytes := func(key string) *int64 {
		value, ok := hostField(meminfo, key, ':')
		if !ok {
			return nil
		}
		number, _, _ := strings.Cut(value, " ")
		kb, err := strconv.ParseInt(number, 10, 64)
		if !isHostDigits(number) || err != nil {
			return nil
		}
		bytes := kb * 1024
		return &bytes
	}
	reading := HostMemoryReading{Total: kilobytes("MemTotal"), Available: kilobytes("MemAvailable"), Free: kilobytes("MemFree")}
	buffers, cached := kilobytes("Buffers"), kilobytes("Cached")
	if buffers != nil || cached != nil {
		var sum int64
		if buffers != nil {
			sum += *buffers
		}
		if cached != nil {
			sum += *cached
		}
		reading.Cached = &sum
	}
	return reading
}

// LinuxUptime 由 /proc/uptime 的内容读出系统运行时长：第一列（到第一个空格或换行为止）是秒数，写成十进制小数——只有数字与至多
// 一个小数点，至少一位数字；按十进制读，秒以下取到 100 纳秒。带符号、指数或空白的写法，以及时长类型装不下的值返回假。
func LinuxUptime(uptime string) (time.Duration, bool) {
	column := uptime
	if end := strings.IndexAny(uptime, " \n"); end >= 0 {
		column = uptime[:end]
	}
	whole, fraction, _ := strings.Cut(column, ".")
	if (whole == "" && fraction == "") || (whole != "" && !isHostDigits(whole)) || (fraction != "" && !isHostDigits(fraction)) {
		return 0, false
	}
	var seconds int64
	if whole != "" {
		parsed, err := strconv.ParseInt(whole, 10, 64)
		if err != nil || parsed > int64((1<<63-1)/time.Second)-1 {
			return 0, false
		}
		seconds = parsed
	}
	// 秒以下取七位（100 纳秒），不足的补 0，多出的舍去
	fraction = (fraction + "0000000")[:7]
	ticks, _ := strconv.ParseInt(fraction, 10, 64)
	return time.Duration(seconds)*time.Second + time.Duration(ticks)*100*time.Nanosecond, true
}

// HostMount 是挂载表里的一行：设备、挂载点与文件系统。
type HostMount struct {
	Device     string
	MountPoint string
	FileSystem string
}

// LinuxBlockDeviceMounts 由 /proc/self/mounts 的内容给出要读容量的挂载点：每行按单个空格分列，至少三列（设备、挂载点、文件系统），
// 设备与挂载点里的三位八进制转义（\040 是空格）还原；只取设备以 /dev/ 开头、不以 /dev/loop 开头的行，每个设备取第一行，按文件里的次序。
func LinuxBlockDeviceMounts(mounts string) []HostMount {
	result := []HostMount{}
	seen := map[string]bool{}
	for line := range strings.SplitSeq(mounts, "\n") {
		columns := strings.Split(line, " ")
		if line == "" || len(columns) < 3 {
			continue
		}
		device := unescapeMountText(columns[0])
		if !strings.HasPrefix(device, "/dev/") || strings.HasPrefix(device, "/dev/loop") || seen[device] {
			continue
		}
		seen[device] = true
		result = append(result, HostMount{Device: device, MountPoint: unescapeMountText(columns[1]), FileSystem: columns[2]})
	}
	return result
}

// unescapeMountText 还原挂载表里的八进制转义：反斜杠后面正好跟三位八进制数字时换成那个码位的字符，别的反斜杠原样保留。
func unescapeMountText(text string) string {
	if !strings.Contains(text, "\\") {
		return text
	}
	isOctal := func(char byte) bool { return char >= '0' && char <= '7' }
	var builder strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] == '\\' && index+3 < len(text) && isOctal(text[index+1]) && isOctal(text[index+2]) && isOctal(text[index+3]) {
			builder.WriteRune(rune(text[index+1]-'0')*64 + rune(text[index+2]-'0')*8 + rune(text[index+3]-'0'))
			index += 3
			continue
		}
		builder.WriteByte(text[index])
	}
	return builder.String()
}

// hostLine 读 files 里一个文件的第一行；没有这个文件或清理之后为空返回假。
func hostLine(files map[string]string, path string) (string, bool) {
	text, ok := files[path]
	if !ok {
		return "", false
	}
	return hostFirstLine(text)
}

// hostNumber 读 files 里一个只有一个十进制整数（可以带符号）的文件。
func hostNumber(files map[string]string, path string) (int64, bool) {
	line, ok := hostLine(files, path)
	if !ok {
		return 0, false
	}
	value, err := strconv.ParseInt(line, 10, 64)
	return value, err == nil
}

// hostDirectories 给出 files 里 directory 之下的子目录名，按字节序：路径里 directory 之后还有下一层的那些名字。
func hostDirectories(files map[string]string, directory string) []string {
	seen := map[string]bool{}
	for path := range files {
		rest, ok := strings.CutPrefix(path, directory+"/")
		if !ok {
			continue
		}
		if end := strings.IndexByte(rest, '/'); end > 0 {
			seen[rest[:end]] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// isNumbered 判断 name 是否是 prefix 加上至少一位十进制数字（cpu0、card1）。
func isNumbered(name, prefix string) bool {
	rest, ok := strings.CutPrefix(name, prefix)
	return ok && isHostDigits(rest)
}

// HostProcessorReading 是读到的处理器静态信息，nil 表示读不到：型号、厂商、物理核心数、标称频率（MHz）与最后一级缓存的字节数。
type HostProcessorReading struct {
	Name          *string
	Vendor        *string
	PhysicalCores *int
	FrequencyMHz  *float64
	CacheBytes    *int64
}

// linuxCPUDirectory 是处理器的 sysfs 目录（相对文件系统的根）。
const linuxCPUDirectory = "sys/devices/system/cpu"

// LinuxProcessor 由 /proc/cpuinfo 与 /sys/devices/system/cpu 下的文件读出处理器的静态信息；files 是路径（相对文件系统的根，
// 不带开头的斜杠）到文件内容。型号取 cpuinfo 里第一个 model name（没有或为空取 Hardware），厂商取 vendor_id；物理核心数是各
// cpu<N>/topology 里 physical_package_id 与 core_id 的不同组合数；标称频率取 cpu0/cpufreq/base_frequency（没有取
// cpuinfo_max_freq），千赫除以 1000；最后一级缓存取 cpu0/cache/index<N> 里级别最高、不是指令缓存的那一项的大小。
func LinuxProcessor(files map[string]string) HostProcessorReading {
	var reading HostProcessorReading
	if cpuinfo, ok := files["proc/cpuinfo"]; ok {
		if name, ok := hostField(cpuinfo, "model name", ':'); ok {
			reading.Name = &name
		} else if name, ok := hostField(cpuinfo, "Hardware", ':'); ok {
			reading.Name = &name
		}
		if vendor, ok := hostField(cpuinfo, "vendor_id", ':'); ok {
			reading.Vendor = &vendor
		}
	}

	cores := map[[2]int64]bool{}
	for _, name := range hostDirectories(files, linuxCPUDirectory) {
		if !isNumbered(name, "cpu") {
			continue
		}
		pack, okPackage := hostNumber(files, linuxCPUDirectory+"/"+name+"/topology/physical_package_id")
		core, okCore := hostNumber(files, linuxCPUDirectory+"/"+name+"/topology/core_id")
		if okPackage && okCore {
			cores[[2]int64{pack, core}] = true
		}
	}
	if count := len(cores); count > 0 {
		reading.PhysicalCores = &count
	}

	kilohertz, ok := hostNumber(files, linuxCPUDirectory+"/cpu0/cpufreq/base_frequency")
	if !ok {
		kilohertz, ok = hostNumber(files, linuxCPUDirectory+"/cpu0/cpufreq/cpuinfo_max_freq")
	}
	if ok {
		megahertz := float64(kilohertz) / 1000
		reading.FrequencyMHz = &megahertz
	}

	// 级别要严格更高才换：同一级有几项时留下目录名按字节序排在最前的那一项
	var level int64
	for _, name := range hostDirectories(files, linuxCPUDirectory+"/cpu0/cache") {
		if !strings.HasPrefix(name, "index") {
			continue
		}
		prefix := linuxCPUDirectory + "/cpu0/cache/" + name
		current, ok := hostNumber(files, prefix+"/level")
		if !ok || current <= level {
			continue
		}
		if kind, _ := hostLine(files, prefix+"/type"); kind == "Instruction" {
			continue
		}
		text, ok := hostLine(files, prefix+"/size")
		if !ok {
			continue
		}
		if size, ok := parseHostCacheSize(text); ok {
			level, reading.CacheBytes = current, &size
		}
	}
	return reading
}

// parseHostCacheSize 把缓存大小的文本换成字节数：结尾的 K、M、G（不区分大小写）是 1024 的一、二、三次方，没有单位就是字节数；
// 单位之前必须是十进制数字。
func parseHostCacheSize(text string) (int64, bool) {
	multiplier := int64(1)
	switch text[len(text)-1] {
	case 'K', 'k':
		multiplier = 1 << 10
	case 'M', 'm':
		multiplier = 1 << 20
	case 'G', 'g':
		multiplier = 1 << 30
	}
	if multiplier != 1 {
		text = text[:len(text)-1]
	}
	number, err := strconv.ParseInt(text, 10, 64)
	if !isHostDigits(text) || err != nil {
		return 0, false
	}
	return number * multiplier, true
}

// HostGPUReading 是读到的一块显卡，nil 表示读不到：名称、厂商名、四位小写十六进制的 PCI 厂商与设备 ID、驱动与驱动版本、
// 显存字节数、温度（摄氏度）。
type HostGPUReading struct {
	Name               *string
	Vendor             *string
	VendorID           *string
	DeviceID           *string
	Driver             *string
	DriverVersion      *string
	MemoryBytes        *int64
	TemperatureCelsius *float64
}

// linuxPCIIDPaths 是 pci.ids 的候选位置，用第一个存在的。
var linuxPCIIDPaths = []string{"usr/share/hwdata/pci.ids", "usr/share/misc/pci.ids", "usr/share/pci.ids"}

// LinuxGPUs 由 /sys/class/drm 下的文件读出各块显卡，按 card<N> 目录名的字节序；files 是路径（相对文件系统的根）到文件内容。
// 驱动与 PCI ID 取 card<N>/device/uevent 的 DRIVER 与 PCI_ID（没有 PCI_ID 取 vendor 与 device 两个文件），名称与厂商名查
// pci.ids，驱动版本取 /sys/module/<驱动>/version，显存取 mem_info_vram_total，温度取 hwmon 下第一个读得出的 temp1_input（毫摄氏度）。
func LinuxGPUs(files map[string]string) []HostGPUReading {
	gpus := []HostGPUReading{}
	for _, card := range hostDirectories(files, "sys/class/drm") {
		if !isNumbered(card, "card") {
			continue
		}
		device := "sys/class/drm/" + card + "/device"
		uevent := files[device+"/uevent"]
		var gpu HostGPUReading
		if driver, ok := hostField(uevent, "DRIVER", '='); ok {
			gpu.Driver = &driver
			if version, ok := hostLine(files, "sys/module/"+driver+"/version"); ok {
				gpu.DriverVersion = &version
			}
		}

		// PCI_ID 正好是冒号分开的两段时用它，哪一段不合写法哪一段为空；否则用 vendor 与 device 两个文件
		vendorText, _ := hostLine(files, device+"/vendor")
		deviceText, _ := hostLine(files, device+"/device")
		if pciID, ok := hostField(uevent, "PCI_ID", '='); ok {
			if parts := strings.Split(pciID, ":"); len(parts) == 2 {
				vendorText, deviceText = parts[0], parts[1]
			}
		}
		gpu.VendorID, gpu.DeviceID = normalizePCIID(vendorText), normalizePCIID(deviceText)
		gpu.Vendor, gpu.Name = findPCINames(files, gpu.VendorID, gpu.DeviceID)

		if memory, ok := hostNumber(files, device+"/mem_info_vram_total"); ok {
			gpu.MemoryBytes = &memory
		}
		for _, sensor := range hostDirectories(files, device+"/hwmon") {
			if millidegrees, ok := hostNumber(files, device+"/hwmon/"+sensor+"/temp1_input"); ok {
				celsius := float64(millidegrees) / 1000
				gpu.TemperatureCelsius = &celsius
				break
			}
		}
		gpus = append(gpus, gpu)
	}
	return gpus
}

// normalizePCIID 把 PCI 的厂商或设备 ID 规范成四位小写十六进制：开头的 0x 去掉，其余必须正好是四位十六进制数字，否则为空。
func normalizePCIID(text string) *string {
	if len(text) >= 2 && text[0] == '0' && (text[1] == 'x' || text[1] == 'X') {
		text = text[2:]
	}
	if len(text) != 4 {
		return nil
	}
	for index := 0; index < len(text); index++ {
		char := text[index]
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f' || char >= 'A' && char <= 'F') {
			return nil
		}
	}
	id := strings.ToLower(text)
	return &id
}

// findPCINames 在 pci.ids 里查厂商名与设备名：用候选位置里第一个存在的文件。厂商行顶格，设备行缩进一个制表符，缩进两个制表符的
// 子系统行与 C 加空格起头的类别段不看；空行与 # 起头的行跳过。厂商查得到而它名下没有这个设备时只有厂商名。
func findPCINames(files map[string]string, vendorID, deviceID *string) (vendor, device *string) {
	if vendorID == nil {
		return nil, nil
	}
	for _, path := range linuxPCIIDPaths {
		text, ok := files[path]
		if !ok {
			continue
		}
		for line := range strings.SplitSeq(text, "\n") {
			line = strings.TrimSuffix(line, "\r")
			if line == "" || line[0] == '#' {
				continue
			}
			if line[0] != '\t' {
				// 下一个厂商或类别段：已经找到的厂商名下没有这个设备
				if vendor != nil || strings.HasPrefix(line, "C ") {
					break
				}
				vendor = pciEntryName(line, *vendorID)
				continue
			}
			if vendor != nil && deviceID != nil && len(line) > 1 && line[1] != '\t' {
				if name := pciEntryName(line[1:], *deviceID); name != nil {
					return vendor, name
				}
			}
		}
		return vendor, nil
	}
	return nil, nil
}

// pciEntryName 在行首的四位 ID 相符（不区分大小写）、第五个字符是空格、整行多于六个字符时给出之后的名称；名称清理之后为空返回 nil。
func pciEntryName(line, id string) *string {
	if utf8.RuneCountInString(line) <= 6 || len(line) < 5 || !strings.EqualFold(line[:4], id) || line[4] != ' ' {
		return nil
	}
	if name, ok := cleanHostText(line[4:]); ok {
		return &name
	}
	return nil
}
