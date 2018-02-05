// Package uconf use to parse config file,config file's format just
// like haproxy.conf
//
// configure file example
// <test.cfg>
// global
// 	maxconn 50000
// 	daemon
// 	stats socket /var/run/haproxy.stat mode 777
// defaults
// 	stats enable
// 	option httpchk HEAD /haproxy?monitor HTTP/1.0
// 	timeout check 5s
// listen s 0.0.0.0:80
// 	monitor-uri /haproxy?monitor
// 	monitor fail if
// 	server 10.0.0.x:80 10.0.0.x:80 maxconn 25 check inter 5s rise 3 fall 2
// 	acl servers_down nbsrv(servers) lt 1
// listen s 0.0.0.0:443
// 	mode tcp
// 	option ssl-hello-chk
// 	server server1 10.0.0.x:443 maxconn 25 check inter 5s rise 18 fall 3
// 	server server2 10.0.0.x:443 maxconn 25 check inter 4s rise 8 fall 2
//
// example:
// package main

// import (
// 	"fmt"

// 	"github.com/hxgz/uconf"
// )

// func main() {
// 	cf := uconf.NewConfigFile()
// 	cf.SetSectionName([]string{"global", "defaults", "listen", ""}...)
// 	cf.LoadFile("test.cfg")
// 	//cf.PrintConf()
// 	//list section
// 	for name, sections := range cf.GetALLSection() {
// 		fmt.Println("section:", name)
// 		for _, section := range sections {
// 			for _, value := range section.GetAllSliceValue() {
// 				fmt.Printf("%s ", value)
// 			}
// 		}
// 		fmt.Printf("\n")
// 	}
// 	fmt.Printf("\n")
// 	//get one section
// 	s, err := cf.GetSectionIndex("listen", 0)
// 	if err != nil {
// 		return
// 	}
// 	fmt.Println("Get value from key:", s.GetValue("s"))
// 	fmt.Println("Get value from index:", s.GetValueIndex(1))
// 	//get keys
// 	keys, err := cf.GetKeysIndex("listen", 1)
// 	if err != nil {
// 		return
// 	}
// 	//for key, values := range keys {
// 	values := keys["server"]
// 	fmt.Println("section:listen,key:server")
// 	for index, value := range values {
// 		fmt.Printf("server index:%d,Get inter param:%s\n", index, value.GetValue("inter"))
// 	}
// }
package uconf

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
)

const (
	// Default section name.
	DEFAULT_SECTION = "DEFAULT"
)

//Property store each line
type Property struct {
	name       string            //Just some
	avals      []string          //list of all vlaues
	kwvals     map[string]string //dictionary of values
	keycomment string            //property comment
}

//return a new Property object
func newProperty(name string) *Property {
	p := &Property{
		name:   name,
		kwvals: make(map[string]string),
	}
	return p
}

//Load parse raw string get from file
//the first word will be the name just like key
//string after char [;#] will be comment
//return false when string is empty after strip the comment
//otherwise true
func (p *Property) Load(str string) bool {
	//get content and comment
	var content string
	reg := regexp.MustCompile("(?P<content>[^;#]*)[;|#](?P<comment>.*)$")
	match := reg.FindStringSubmatch(str)
	if match == nil {
		content = str
	} else {
		content = match[1]
		p.keycomment = match[2]
	}
	length := len(strings.Fields(content))
	if length == 0 {
		return false
	}
	//write content
	p.name = strings.Fields(content)[0]
	values := strings.Fields(content)[1:]
	p.AppendList(values...)
	for i := length - 2; i > 0; i -= 2 {
		p.kwvals[values[i-1]] = values[i]
	}
	return true
}

//apend the value to p.avals
//param:value can be a slice of string or just string
func (p *Property) AppendList(value ...string) {
	p.avals = append(p.avals, value...)
}

//set value to p.kwvals
func (p *Property) SetValue(key, value string) {
	p.kwvals[key] = value
}

//set comment
func (p *Property) SetComments(name, comments string) {
	p.keycomment = comments
}

//Get all the value after name
//return a slice of value.
func (p *Property) GetAllSliceValue() []string {
	return p.avals
}

//Just Like GetAllSliceValue,but return a map
func (p *Property) GetAllMapValue() map[string]string {
	return p.kwvals
}

//Get value
func (p *Property) GetValue(key string) string {
	values := p.GetAllMapValue()
	return values[key]
}

//Get value
func (p *Property) GetValueIndex(index int) string {
	values := p.GetAllSliceValue()
	return values[index]
}

//print the config
func (p *Property) Print() {
	fmt.Printf("%s ", p.name)
	for _, value := range p.avals {
		fmt.Printf("%s ", value)

	}
	if p.keycomment != "" {
		fmt.Printf("#%s ", p.keycomment)
	}
	fmt.Printf("\n")
}

// A ConfigFile represents configuration file like haproxy.conf.
type ConfigFile struct {
	lock        sync.RWMutex                       // Go map is not safe.
	fileNames   []string                           // filename list
	data        map[string][]map[string][]Property // Section -> [{"name":[Property1,Property2]},,...]
	sectionData map[string][]Property              //section name -> [Property1,Property2,...]

	sectionList    []string              // Section name list.
	CurrentKeys    map[string][]Property //where key stored recently
	CurrentSection *Property             //section updated recently
}

// Get a new ConfigFile
func NewConfigFile() *ConfigFile {
	c := new(ConfigFile)
	c.data = make(map[string][]map[string][]Property)
	c.sectionData = make(map[string][]Property)

	return c
}

//Print the config data to stdout
func (c *ConfigFile) PrintConf() {
	for name, sections := range c.sectionData {
		// for _, name := range c.sectionList {
		// 	sections := c.sectionData[name]
		for index, section := range sections {
			section.Print()
			for _, keys := range c.data[name][index] {
				for _, key := range keys {
					fmt.Printf("\t")
					key.Print()
				}
			}
		}
	}
}

//Set the Section name,multiple name can be support
//If caller donot use the function,a default name<DEFAULT_SECTION> will be use
func (c *ConfigFile) SetSectionName(sections ...string) {
	c.sectionList = append(c.sectionList, sections...)
}

//Get all sections
//name:[section1,section2,....]
func (c *ConfigFile) GetALLSection() map[string][]Property {
	return c.sectionData
}

//Get section according name
func (c *ConfigFile) GetSection(name string) ([]Property, error) {
	s := c.sectionData[name]
	if s == nil {
		return nil, errors.New("section not found")
	}
	return c.sectionData[name], nil
}

//Get section according name,index
func (c *ConfigFile) GetSectionIndex(name string, index int) (*Property, error) {
	s, err := c.GetSection(name)
	if err != nil {
		return nil, err
	}
	if len(s) < index {
		return nil, errors.New("index out of range")
	}
	return &s[index], nil
}

//add section data
//section: section name
//prop:section Property object
func (c *ConfigFile) AddSectionValue(section string, prop Property) {
	//update sectiondata
	if _, ok := c.sectionData[section]; !ok {
		c.sectionData[section] = make([]Property, 0)
		c.data[section] = make([]map[string][]Property, 0)
	}
	key := make(map[string][]Property)
	c.sectionData[section] = append(c.sectionData[section], prop)
	c.data[section] = append(c.data[section], key)
	c.CurrentSection = &prop
	c.CurrentKeys = key
}

//Set key or options
//prop: key Property object
func (c *ConfigFile) SetKey(prop Property) {
	//check if set section
	if c.CurrentKeys == nil {
		c.SetSectionName(DEFAULT_SECTION)
		c.AddSectionValue(DEFAULT_SECTION, *newProperty(DEFAULT_SECTION))
	}
	keys := c.CurrentKeys
	if _, ok := keys[prop.name]; !ok {
		keys[prop.name] = make([]Property, 0)
	}
	keys[prop.name] = append(keys[prop.name], prop)
}

//Get all keys belong to section
//[key_name:[key1,key2,....],......]
func (c *ConfigFile) GetALLKeys(section string) ([]map[string][]Property, error) {
	keys, ok := c.data[section]
	if !ok {
		return nil, errors.New("key not found")
	}

	return keys, nil
}

//Get key accroding index belong to section
//{"name":[key1,key2,...]}
func (c *ConfigFile) GetKeysIndex(section string, index int) (map[string][]Property, error) {
	keys, err := c.GetALLKeys(section)
	if err != nil {
		return nil, err
	}
	if len(keys) < index {
		return nil, errors.New("index out of range")
	}
	return keys[index], nil
}

//parse a raw line,and choose it as section or key
func (c *ConfigFile) LoadString(line string) {
	p := newProperty("")
	if ok := p.Load(line); !ok {
		return
	}

	section_name := p.name
	//c.SetSectionName(section_name)
	if contains(c.sectionList, section_name) {
		c.AddSectionValue(section_name, *p)
	} else {
		c.SetKey(*p)
	}
}

//parse the whole file,multiple files can support
func (c *ConfigFile) LoadFile(fileName ...string) (err error) {
	c.fileNames = append(c.fileNames, fileName...)
	for _, fileName := range c.fileNames {
		f, err := os.Open(fileName)
		if err != nil {
			return err
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			c.LoadString(scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}
	return
}

// Reload configuration file if some file changed
func (c *ConfigFile) Reload() (err error) {
	cfg := NewConfigFile()
	err = cfg.LoadFile(c.fileNames...)
	if err == nil {
		*c = *cfg
	}
	return err
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
