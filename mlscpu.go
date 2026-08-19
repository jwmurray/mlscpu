package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/iancoleman/orderedmap"
)

const Cmds = `{
    "Architecture": "uname -m",
    "Byte Order": "sysctl -n hw.byteorder",
    "CPU(s)": "sysctl -n hw.ncpu",
    "On-line CPU(s)": "sysctl -n hw.activecpu",
    "Thread(s) per core": "echo \"scale=0; $(sysctl -n machdep.cpu.thread_count) / $(sysctl -n machdep.cpu.core_count)\" | bc",
    "Core(s) per socket": "echo \"scale=0; $(sysctl -n machdep.cpu.core_count) / $(sysctl -n hw.packages)\" | bc",
    "Socket(s)": "sysctl -n hw.packages",
    "Vendor ID": "sysctl -n machdep.cpu.vendor",
    "CPU family": "sysctl -n machdep.cpu.family",
    "CPU Model": "sysctl -n machdep.cpu.model",
    "Model name": "sysctl -n machdep.cpu.brand_string",
    "Stepping": "sysctl -n machdep.cpu.stepping",
    "CPU MHz": "sysctl -n hw.cpufrequency",
    "CPU max MHz": "sysctl -n hw.cpufrequency_max",
    "CPU min MHz": "sysctl -n hw.cpufrequency_min",
    "Hyper-Threading Technology": "system_profiler SPHardwareDataType | grep \"Hyper-Threading Technology\" | cut -d: -f2-",
    "L1d cache": "sysctl -n hw.l1dcachesize",
    "L1i cache": "sysctl -n hw.l1icachesize",
    "L2 cache": "sysctl -n hw.l2cachesize",
    "L3 cache": "sysctl -n hw.l3cachesize",
    "Flags": "sysctl -n machdep.cpu.features"
}
`

// https://stackoverflow.com/a/15323988/7543474
// Licensed under CC BY-SA 4.0
func string_in_slice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func modify_cmd_output(cmd string, cmd_output string) string {
	if cmd == "Byte Order" {
		if cmd_output == "1234" {
			return "Little Endian"
		} else if cmd_output == "4321" {
			return "Big Endian"
		}
	} else if cmd == "CPU MHz" || cmd == "CPU max MHz" || cmd == "CPU min MHz" {
		mhz, err := strconv.Atoi(cmd_output)
		if err != nil {
			log.Println("Error converting CPU MHz to int: ", err)
		}
		return strconv.Itoa(mhz / 1000000)
	} else if cmd == "L1d cache" || cmd == "L1i cache" || cmd == "L2 cache" || cmd == "L3 cache" {
		cache_size, err := strconv.Atoi(cmd_output)
		if err != nil {
			log.Println("Error converting cache size to int: ", err)
		}
		return strconv.Itoa(cache_size/1024) + "K"
	}
	return cmd_output
}

func decode_json_file() *orderedmap.OrderedMap {
	decoder := json.NewDecoder(strings.NewReader(Cmds))
	commands := orderedmap.New()
	decoder.UseNumber()
	err := decoder.Decode(&commands)
	if err != nil {
		log.Println("Error decoding JSON file: ", err)
		return nil
	}
	return commands
}

// ---------------------------------------------------------------------------
// Apple Silicon
//
// None of the machdep.cpu.* keys the Intel path reads (vendor, family, model,
// stepping, features) exist on arm64, and neither do hw.cpufrequency* or
// hw.l3cachesize. The equivalent data lives in the hw.* tree instead, so arm64
// gets its own field set rather than printing a column of empty values.
// ---------------------------------------------------------------------------

// A perf level is one cluster type (Performance, Efficiency, ...). Apple
// Silicon reports cores and caches per level; the flat hw.l1dcachesize and
// friends only describe one of them, so reading those alone is misleading.
type perf_level struct {
	name  string
	cores int64
	l1d   int64
	l1i   int64
	l2    int64
}

func sysctl(key string) (string, bool) {
	output, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(output)), true
}

func sysctl_int(key string) (int64, bool) {
	value, ok := sysctl(key)
	if !ok {
		return 0, false
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return number, true
}

// hw.optional.arm64 describes the machine rather than the running binary, so
// this still detects Apple Silicon when an amd64 build runs under Rosetta.
func is_apple_silicon() bool {
	value, ok := sysctl("hw.optional.arm64")
	return ok && value == "1"
}

func kilobytes(bytes int64) string {
	return strconv.FormatInt(bytes/1024, 10) + "K"
}

func read_perf_levels() []perf_level {
	count, ok := sysctl_int("hw.nperflevels")
	if !ok {
		return nil
	}

	levels := []perf_level{}
	for index := int64(0); index < count; index++ {
		prefix := fmt.Sprintf("hw.perflevel%d.", index)
		level := perf_level{}
		level.name, _ = sysctl(prefix + "name")
		level.cores, _ = sysctl_int(prefix + "physicalcpu")
		level.l1d, _ = sysctl_int(prefix + "l1dcachesize")
		level.l1i, _ = sysctl_int(prefix + "l1icachesize")
		level.l2, _ = sysctl_int(prefix + "l2cachesize")
		levels = append(levels, level)
	}
	return levels
}

// Renders one value per perf level, qualified by level name. A machine with a
// single level needs no qualifier, so it degrades to a plain value.
func per_level(levels []perf_level, value func(perf_level) string) string {
	parts := []string{}
	for _, level := range levels {
		if len(levels) == 1 || level.name == "" {
			parts = append(parts, value(level))
		} else {
			parts = append(parts, fmt.Sprintf("%s (%s)", value(level), level.name))
		}
	}
	return strings.Join(parts, ", ")
}

// The ARM equivalent of machdep.cpu.features: every hw.optional.* key set to 1.
// Both the modern hw.optional.arm.FEAT_* names and the older top-level aliases
// (neon, armv8_1_atomics, ...) are reported, since CPU tools differ over which
// spelling they use and the union is what a reader is likely to grep for. Keys
// holding a non-boolean value (caps, watchpoint, sme_max_svl_b) and features
// reported as unsupported both fall out of the value != "1" test.
func arm_flags() string {
	output, err := exec.Command("sysctl", "hw.optional").Output()
	if err != nil {
		log.Println("Error reading ARM feature flags: ", err)
		return ""
	}

	seen := map[string]bool{}
	flags := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		key, value, found := strings.Cut(line, ": ")
		if !found || value != "1" {
			continue
		}

		key = strings.TrimPrefix(strings.TrimSpace(key), "hw.optional.")
		key = strings.TrimPrefix(key, "arm.")
		flag := strings.ToLower(strings.TrimPrefix(key, "FEAT_"))

		// hw.optional.arm64 restates the architecture rather than naming a
		// CPU feature, and is already reported as its own field.
		if flag == "arm64" || seen[flag] {
			continue
		}
		seen[flag] = true
		flags = append(flags, flag)
	}

	sort.Strings(flags)
	return strings.Join(flags, " ")
}

func apple_silicon_info() *orderedmap.OrderedMap {
	info := orderedmap.New()

	// Only records fields the machine actually reports, so a missing sysctl
	// is omitted rather than printed as a blank line.
	set := func(label string, value string, ok bool) {
		if ok && value != "" {
			info.Set(label, value)
		}
	}
	set_int := func(label string, key string) {
		value, ok := sysctl(key)
		set(label, value, ok)
	}

	architecture, ok := sysctl("hw.machine")
	set("Architecture", architecture, ok)

	byte_order, ok := sysctl("hw.byteorder")
	set("Byte Order", modify_cmd_output("Byte Order", byte_order), ok)

	set_int("CPU(s)", "hw.ncpu")
	set_int("On-line CPU(s)", "hw.activecpu")

	threads, threads_ok := sysctl_int("machdep.cpu.thread_count")
	cores, cores_ok := sysctl_int("machdep.cpu.core_count")
	sockets, sockets_ok := sysctl_int("hw.packages")
	if threads_ok && cores_ok && cores > 0 {
		set("Thread(s) per core", strconv.FormatInt(threads/cores, 10), true)
	}
	if cores_ok && sockets_ok && sockets > 0 {
		set("Core(s) per socket", strconv.FormatInt(cores/sockets, 10), true)
	}
	if sockets_ok {
		set("Socket(s)", strconv.FormatInt(sockets, 10), true)
	}

	set("Vendor ID", "Apple", true)
	set_int("Model name", "machdep.cpu.brand_string")

	// hw.cpufamily is a 32-bit hash printed signed; hex matches how Apple
	// documents the family constants.
	if family, ok := sysctl_int("hw.cpufamily"); ok {
		set("CPU family", fmt.Sprintf("0x%08x", uint32(family)), true)
	}
	set_int("CPU subfamily", "hw.cpusubfamily")

	levels := read_perf_levels()
	if len(levels) > 0 {
		set("Core(s) per perf level", per_level(levels, func(level perf_level) string {
			return strconv.FormatInt(level.cores, 10)
		}), true)
		set("L1d cache", per_level(levels, func(level perf_level) string {
			return kilobytes(level.l1d)
		}), true)
		set("L1i cache", per_level(levels, func(level perf_level) string {
			return kilobytes(level.l1i)
		}), true)
		set("L2 cache", per_level(levels, func(level perf_level) string {
			return kilobytes(level.l2)
		}), true)
	}

	if line_size, ok := sysctl_int("hw.cachelinesize"); ok {
		set("Cache line size", strconv.FormatInt(line_size, 10)+"B", true)
	}

	set("Flags", arm_flags(), true)
	return info
}

func print_info(info *orderedmap.OrderedMap) {
	for _, label := range info.Keys() {
		value, _ := info.Get(label)
		fmt.Printf("%s: %s\n", label, value)
	}
}

func main() {
	if is_apple_silicon() {
		print_info(apple_silicon_info())
		return
	}

	commands := decode_json_file()
	if commands == nil {
		return
	}

	for _, cmd_name := range commands.Keys() {
		cmd, _ := commands.Get(cmd_name)
		output, err := []byte{}, error(nil)

		if strings.Contains(cmd.(string), "|") {
			output, err = exec.Command("sh", "-c", cmd.(string)).Output()
		} else {
			sub_commands := strings.Split(cmd.(string), " ")
			output, err = exec.Command(sub_commands[0], sub_commands[1:]...).Output()
		}
		if err != nil {
			log.Println("Error executing command ", cmd, " : ", err)
		}

		cmd_name = strings.TrimSpace(cmd_name)
		output_str := strings.TrimSpace(strings.TrimRight(string(output), "\n"))
		output_str = modify_cmd_output(cmd_name, output_str)
		fmt.Printf("%s: %s\n", cmd_name, output_str)
	}
}
