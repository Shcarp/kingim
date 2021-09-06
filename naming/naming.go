package naming

import "kingim"

type Naming interface {
	Find(serviceName string, tags ...string) ([]kingim.ServiceRegistration, error)
	Subscribe(serviceName string, callback func(services []kingim.ServiceRegistration)) error
	Unsubscribe(serviceName string) error
	Register(service kingim.ServiceRegistration) error
	Deregister(serviceID string) error
}


