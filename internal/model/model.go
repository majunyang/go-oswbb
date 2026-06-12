package model

import "time"

type ChartSeries struct {
	Name string    `json:"name"`
	Data []float64 `json:"data"`
}

type ChartResponse struct {
	Series     []ChartSeries `json:"series"`
	Categories []string      `json:"categories"`
	Interfaces []string      `json:"interfaces,omitempty"`
	Devices    []string      `json:"devices,omitempty"`
	CPUs       []string      `json:"cpus,omitempty"`
}

// Vmstat

type VmstatMetadata struct {
	Version     string
	SnapInterval int
	CPUCores     int
	VCPUs        int
}

type VmstatProcsRecord struct {
	Timestamp time.Time `json:"timestamp"`
	R int `json:"r"`
	B int `json:"b"`
}

type VmstatMemoryRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Free  int `json:"free"`
	Buff  int `json:"buff"`
	Cache int `json:"cache"`
}

type VmstatSwapRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Swpd int `json:"swpd"`
	Si   int `json:"si"`
	So   int `json:"so"`
}

type VmstatIORecord struct {
	Timestamp time.Time `json:"timestamp"`
	Bi int `json:"bi"`
	Bo int `json:"bo"`
}

type VmstatSystemRecord struct {
	Timestamp time.Time `json:"timestamp"`
	In int `json:"in"`
	Cs int `json:"cs"`
}

type VmstatCPURecord struct {
	Timestamp time.Time `json:"timestamp"`
	Us int `json:"us"`
	Sy int `json:"sy"`
	Id int `json:"id"`
	Wa int `json:"wa"`
	St int `json:"st"`
}

type VmstatParsedData struct {
	Metadata   VmstatMetadata
	Timestamps []time.Time
	Procs      []VmstatProcsRecord
	Memory     []VmstatMemoryRecord
	Swap       []VmstatSwapRecord
	IO         []VmstatIORecord
	System     []VmstatSystemRecord
	CPU        []VmstatCPURecord
}

// Iostat

type IostatRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Device    string    `json:"device"`
	RS        float64   `json:"rs"`
	WS        float64   `json:"ws"`
	RKBs      float64   `json:"rkBs"`
	WKBs      float64   `json:"wkBs"`
	RRQMs     float64   `json:"rrqms"`
	WRQMs     float64   `json:"wrqms"`
	PRRQM     float64   `json:"prrqm"`
	PWRQM     float64   `json:"pwrqm"`
	RAwait    float64   `json:"rAwait"`
	WAwait    float64   `json:"wAwait"`
	AquSz     float64   `json:"aquSz"`
	RareqSz   float64   `json:"rareqSz"`
	WareqSz   float64   `json:"wareqSz"`
	Svctm     float64   `json:"svctm"`
	Util      float64   `json:"util"`
}

type IostatParsedData struct {
	Timestamps []time.Time
	Devices    map[string][]IostatRecord
}

// Ifconfig

type IfconfigRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	Interface  string    `json:"interface"`
	RxBytes    int64     `json:"rxBytes"`
	TxBytes    int64     `json:"txBytes"`
	RxPackets  int64     `json:"rxPackets"`
	TxPackets  int64     `json:"txPackets"`
	RxErrors   int64     `json:"rxErrors"`
	TxErrors   int64     `json:"txErrors"`
	RxDropped  int64     `json:"rxDropped"`
	TxDropped  int64     `json:"txDropped"`
	RxMissed   int64     `json:"rxMissed"`
}

type IfconfigThroughput struct {
	Timestamp       time.Time `json:"timestamp"`
	RxBytesPerSec   float64   `json:"rxBytesPerSec"`
	TxBytesPerSec   float64   `json:"txBytesPerSec"`
	RxPacketsPerSec float64   `json:"rxPacketsPerSec"`
	TxPacketsPerSec float64   `json:"txPacketsPerSec"`
	RxErrorsPerSec  float64   `json:"rxErrorsPerSec"`
	TxErrorsPerSec  float64   `json:"txErrorsPerSec"`
	RxDroppedPerSec float64   `json:"rxDroppedPerSec"`
	TxDroppedPerSec float64   `json:"txDroppedPerSec"`
	RxMissedPerSec  float64   `json:"rxMissedPerSec"`
}

type IfconfigParsedData struct {
	Timestamps []time.Time
	Interfaces map[string][]IfconfigRecord
}

// Mpstat

type MpstatRecord struct {
	Timestamp time.Time `json:"timestamp"`
	CPUID     string    `json:"cpuId"`
	Usr       float64   `json:"usr"`
	Nice      float64   `json:"nice"`
	Sys       float64   `json:"sys"`
	IOWait    float64   `json:"iowait"`
	IRQ       float64   `json:"irq"`
	Soft      float64   `json:"soft"`
	Steal     float64   `json:"steal"`
	Guest     float64   `json:"guest"`
	GNice     float64   `json:"gnice"`
	Idle      float64   `json:"idle"`
}

type MpstatParsedData struct {
	Timestamps []time.Time
	CPUs       map[string][]MpstatRecord
}

// Meminfo

type MeminfoRecord struct {
	Timestamp      time.Time `json:"timestamp"`
	MemTotal       int64     `json:"memTotal"`
	MemFree        int64     `json:"memFree"`
	Buffers        int64     `json:"buffers"`
	Cached         int64     `json:"cached"`
	SwapTotal      int64     `json:"swapTotal"`
	SwapFree       int64     `json:"swapFree"`
	SwapCached     int64     `json:"swapCached"`
	Active         int64     `json:"active"`
	Inactive       int64     `json:"inactive"`
	ActiveAnon     int64     `json:"activeAnon"`
	InactiveAnon   int64     `json:"inactiveAnon"`
	ActiveFile     int64     `json:"activeFile"`
	InactiveFile   int64     `json:"inactiveFile"`
	Dirty          int64     `json:"dirty"`
	Writeback      int64     `json:"writeback"`
	AnonPages      int64     `json:"anonPages"`
	Mapped         int64     `json:"mapped"`
	Shmem          int64     `json:"shmem"`
	Slab           int64     `json:"slab"`
	SReclaimable   int64     `json:"sReclaimable"`
	SUnreclaim     int64     `json:"sUnreclaim"`
	PageTables     int64     `json:"pageTables"`
	KernelStack    int64     `json:"kernelStack"`
	HugePagesTotal int64     `json:"hugePagesTotal"`
	HugePagesFree  int64     `json:"hugePagesFree"`
	CommittedAS    int64     `json:"committedAS"`
}

type MeminfoParsedData struct {
	Timestamps []time.Time
	Records    []MeminfoRecord
}

// Top

type TopSnapshot struct {
	Timestamp     time.Time `json:"timestamp"`
	Load1         float64   `json:"load1"`
	Load5         float64   `json:"load5"`
	Load15        float64   `json:"load15"`
	TasksTotal    int       `json:"tasksTotal"`
	TasksRunning  int       `json:"tasksRunning"`
	TasksSleeping int       `json:"tasksSleeping"`
	CpuUs         float64   `json:"cpuUs"`
	CpuSy         float64   `json:"cpuSy"`
	CpuNi         float64   `json:"cpuNi"`
	CpuId         float64   `json:"cpuId"`
	CpuWa         float64   `json:"cpuWa"`
	CpuSt         float64   `json:"cpuSt"`
	MemTotal      int64     `json:"memTotal"`
	MemUsed       int64     `json:"memUsed"`
	MemFree       int64     `json:"memFree"`
	SwapTotal     int64     `json:"swapTotal"`
	SwapUsed      int64     `json:"swapUsed"`
	SwapFree      int64     `json:"swapFree"`
}

type TopProcess struct {
	PID     int     `json:"pid"`
	User    string  `json:"user"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	Command string  `json:"command"`
}

type TopParsedData struct {
	Timestamps []time.Time
	Snapshots  []TopSnapshot
	TopProcs   []TopProcess
}
