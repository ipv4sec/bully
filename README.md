## Overview

根据相同的 `Tag` 进行集群内选主


Example:

```
func main() {
	log.SetOutput(ioutil.Discard)

	b, _ := bully.NewParticipant("224.0.0.0:2345", "en0", 6666, "Default", 
	func(slave, master string) {
		fmt.Println(slave, master)
	})
	b.Run(make(chan interface{}))
}
```