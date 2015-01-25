package api

import (
	"log"
	"reflect"
)

type dependency struct {
	// The initial dependency state
	// Type Ptr to Struct, or Ptr to Slice of Struct
	Value reflect.Value

	// Init method and its input
	Method *method
}

type dependencies map[reflect.Type]*dependency

// Create a new Dependencies map
func newDependencies(m reflect.Method, r *resource) (dependencies, error) {
	ds := dependencies{} //make(map[reflect.Type]*dependency)
	err := ds.scanDependencies(m, r)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

// Scan the dependencies of a Method
func (ds dependencies) scanDependencies(m reflect.Method, r *resource) error {
	log.Println("Trying to scan method", m.Type, ds == nil)
	// So we scan all dependencies to create a tree

	for i := 0; i < m.Type.NumIn(); i++ {
		input := m.Type.In(i)

		// Check if this type already exists in the dependencies
		// If it was indexed by another type, this method
		// ensures that it will be indexed for this type too
		_, exist := ds.vaueOf(input)
		if exist {
			//log.Printf("Found dependency %s to use as %s\n", dp.Value, t)
			continue
		}

		// If the required resource is http.ResponseWriter or *http.Request or ID
		// it will be added to context on each request and don't need to be mapped
		if isContextType(input) {
			continue // Not need to be mapped as a dependency
		}

		// Scan this dependency and its dependencies recursively
		// and add it to Dependencies list
		d, err := newDependency(input, r)
		if err != nil {
			return err
		}

		// We should add this dependency before scan its Init Dependencies
		// cause the Init first argument will requires the Resource itself

		ds.add(d)

		// Check if Dependency constructor exists
		init, exists := d.Value.Type().MethodByName("Init")
		if !exists {
			//log.Printf("Type %s doesn't have Init method\n", d.Value.Type())
			continue
		}

		//log.Println("Scanning Init for ", init.Type)

		// Init method should have no return,
		// or return just the resource itself and/or error
		err = isValidInit(init)
		if err != nil {
			return err
		}

		//log.Println("Scan Dependencies for Init method", d.Method.Method.Type)

		err = ds.scanDependencies(init, r)
		if err != nil {
			return err
		}

		// And attach it into the Dependency Method
		d.Method = newMethod(init)
	}
	return nil
}

// Scan the dependencies recursively and add it to the Handler dependencies list
// This method ensures that all dependencies will be present
// when the dependents methods want them
func newDependency(t reflect.Type, r *resource) (*dependency, error) {

	//log.Println("Trying to create a new dependency", t)

	err := isValidDependencyType(t)
	if err != nil {
		return nil, err
	}

	// If this dependency is an Interface,
	// we should search which resource satisfies this Interface in the Resource Tree
	// If this is a Struct, just find for the initial value,
	// if the Struct doesn't exist, create one and return it
	v, err := r.valueOf(t)
	if err != nil {
		return nil, err
	}

	d := &dependency{
		Value:  v,
		Method: nil,
	}

	//log.Printf("Created dependency %s to use as %s\n", v, t)

	return d, nil
}

// Add a new dependency to the Dependencies list
func (ds dependencies) add(d *dependency) {
	log.Println("Adding dependency", d.Value.Type(), ds == nil)
	ds[d.Value.Type()] = d
}

// This method checks if exist an value for the received type
// If it already exist, but its indexed by another type
// it will index for the new type too
func (ds dependencies) vaueOf(t reflect.Type) (*dependency, bool) {

	//log.Println("Dependency: Searching for dependency", t)

	d, exist := ds[t]
	if exist {
		//log.Println("Dependency: Found:", d.Value.Type())
		return d, true
	}

	// Check if one of the dependencies is of this type
	for _, d := range ds {
		if d.isType(t) {
			//log.Println("Dependency: Found out of index", d.Value.Type())

			// Index this dependency with this new type it implements
			ds[t] = d
			return d, true
		}
	}

	//log.Println("Dependency: Not Exist")

	// Not found
	return nil, false
}

// Return true if this Resrouce is from by this Type
func (d *dependency) isType(t reflect.Type) bool {

	if t.Kind() == reflect.Interface {
		return d.Value.Type().Implements(t)
	}

	// The Value stored in Dependency
	// is from Type Ptr to Struct, or Ptr to Slice of Struct
	return d.Value.Type() == ptrOfType(t)
}

// Cosntruct a new dependency in a new memory space with the initial dependency value
func (d *dependency) init() reflect.Value {
	v := reflect.New(d.Value.Type().Elem())
	v.Elem().Set(d.Value.Elem())
	return v
}
