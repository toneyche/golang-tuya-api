// Copyright 2019 py60800.
// Use of this source code is governed by Apache-2 licence
// license that can be found in the LICENSE file.

package tuya

import (
   "encoding/json"
   "fmt"
   "log"
   "sync"
   "time"
)

// data collected from broadcast
type pubAppliance struct {
   GwId       string
   Ip         string
   Active     int
   Ability    int
   Mode       int
   ProductKey string
   Version    string
   Encrypt    bool
}

// configuration data
type configurationData struct {
   Name string
   GwId string
   Type string
   Key  string
   Ip   string //optionnel
}

// the appliance proxies the hardware device
type Appliance struct {
   pubAppliance
   Version    string
   lastUpdate time.Time
   mutex      sync.RWMutex
   // Connection management
   cnxStatus int
   cnxSignal *sync.Cond
   cnxMutex  sync.Mutex
   tcpChan   chan query
   // immutable after configuration
   key    []byte
   device Device
}
type DeviceManager struct {
   sync.Mutex
   collection map[string]*Appliance
   namedColl  map[string]Device
}

func newAppliance() *Appliance {
   d := new(Appliance)
   d.cnxSignal = sync.NewCond(&d.cnxMutex)
   d.cnxStatus = 0
   d.Version = "3.1"
   d.tcpChan = make(chan query,2) // allow limited buffering

   return d
}

func (d *Appliance) GetDevice() Device {
   return d.device
}
func (d *Appliance) update(rd *pubAppliance) {
   d.mutex.Lock()
   defer d.mutex.Unlock()
   d.pubAppliance = *rd
   d.cnxSignal.Broadcast() // Unlock thread waiting for the IP
   d.lastUpdate = time.Now()
}
func (d *Appliance) String() string {
   d.mutex.RLock()
   defer d.mutex.RUnlock()
   return fmt.Sprintf("Appliance[Id:%v IP:%v Product:%v]",
      d.GwId, d.Ip, d.ProductKey)
}

func (dm *DeviceManager) getAppliance(id string) *Appliance {
   d, ok := dm.collection[id]
   if !ok {
      d = newAppliance()
      dm.collection[id] = d
   }
   return d
}
// -------------------------------------------
// 修改configure方法
func (dm *DeviceManager) configure(jdata string) {
   conf := make([]configurationData, 0)
   err := json.Unmarshal([]byte(jdata), &conf)
   if err != nil {
      log.Fatal("Conf error:", err)
   }
   for _, c := range conf {
      if len(c.GwId) == 0 {
         log.Fatal("Conf Id missing")
      }
      d := dm.getAppliance(c.GwId)
      d.GwId = c.GwId
      d.key = []byte(c.Key)
      d.Ip = c.Ip // 从配置文件中直接设置设备IP
      b, ok := makeDevice(c.Type)
      if ok {
         b.configure(d, &c)
         d.device = b
         dm.namedColl[b.Name()] = b
         go d.tcpConnManager(d.tcpChan) // to be run after configuration
      }
      // 更新设备信息
      rd := &pubAppliance{
         GwId:       c.GwId,
         Ip:         c.Ip,
         Active:     1,       // 设为激活状态，可以根据配置文件进行调整
         Ability:    0,       // 根据需要设置设备能力
         Mode:       0,       // 根据需要设置设备模式
         ProductKey: "",      // 根据需要设置产品密钥
         Version:    "3.1",   // 根据需要设置设备版本
         Encrypt:    false,   // 根据需要设置加密选项
      }
 d.update(rd)
}
}

// Device Manager
// -------------------------------------------

func newDeviceManager() *DeviceManager {
   dm := new(DeviceManager)
   dm.collection = make(map[string]*Appliance)
   dm.namedColl = make(map[string]Device)
   //go udpListener(dm)
   return dm
}
func NewDeviceManager(jdata string) *DeviceManager {
   dm := newDeviceManager()
   dm.configure(jdata)
   return dm
}
// -------------------------------------------
func (dm *DeviceManager) applianceUpdate(data []byte) {
   var rd pubAppliance
   je := json.Unmarshal(data, &rd)
   if je != nil {
      log.Print("JSON decode error:", je)
      return
   }
   dm.Lock()
   defer dm.Unlock()
   d := dm.getAppliance(rd.GwId)
   d.update(&rd)
}
// -------------------------------------------
func (dm *DeviceManager) ApplianceCount() int {
   dm.Lock()
   defer dm.Unlock()
   return len(dm.collection)
}
// -------------------------------------------
func (dm *DeviceManager) ApplianceKeys() []string {
   dm.Lock()
   defer dm.Unlock()
   keys := make([]string, 0, len(dm.collection))
   for k := range dm.collection {
      keys = append(keys, k)
   }
   return keys
}
// -------------------------------------------
func (dm *DeviceManager) DeviceKeys() []string {
   dm.Lock()
   defer dm.Unlock()
   keys := make([]string, 0, len(dm.namedColl))
   for k := range dm.namedColl {
      keys = append(keys, k)
   }
   return keys
}
// -------------------------------------------
func (dm *DeviceManager) GetAppliance(key string) (*Appliance, bool) {
   dm.Lock()
   defer dm.Unlock()
   d, ok := dm.collection[key]
   return d, ok
}
// -------------------------------------------
func (dm *DeviceManager) GetDevice(key string) (Device, bool) {
   dm.Lock()
   defer dm.Unlock()
   b, ok := dm.namedColl[key]
   return b, ok
}
