package ginkgo

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/onsi/ginkgo/internal"
	"github.com/onsi/ginkgo/types"
)

/*
The EntryDescription decorator allows you to pass a format string to DescribeTable() and Entry().  This format string is used to generate entry names via:

fmt.Sprintf(formatString, parameters...)

where parameters are the parameters passed into the entry.

When passed into an Entry the EntryDescription is used to generate the name or that entry.  When passed to DescribeTable, the EntryDescription is used to generate teh names for any entries that have `nil` descriptions.
*/
type EntryDescription string

func (ed EntryDescription) render(args ...interface{}) string {
	return fmt.Sprintf(string(ed), args...)
}

/*
DescribeTable describes a table-driven test.

For example:

    DescribeTable("a simple table",
        func(x int, y int, expected bool) {
            Ω(x > y).Should(Equal(expected))
        },
        Entry("x > y", 1, 0, true),
        Entry("x == y", 0, 0, false),
        Entry("x < y", 0, 1, false),
    )

The first argument to `DescribeTable` is a string description.
The subsequent arguments can include the following:
  - a Table Body function that will be run for each table entry.  This function is required and can take any number of parameters but must return nothing.  The parameters associated with each table entry will be passed into this function.  Your assertions go here - the function is equivalent to a Ginkgo It.
  - any Ginkgo decorator (optional)
  - a function that accepts the same parameters as the Table Body function but returns a string.  This function is used to generate the names for entries with nil descriptions.
  - a format string of type EntryDescription.  This format string is used to generate names for entries with nil descriptions.
  - individual table entries.  These are constructed via Entry() and provide parameters for each test case.

The first argument to `Entry` is a description.  This can be a string, an EntryDescription(), a function that returns a string, or nil.  When nil, the Entry's name is generated using the table-level entry description function.  If none is provided than a default name is constructed from the passed-in parameters.
The sbusequent arguments to `Entry` can include any Ginkgo decorators.  These are filtered out and applied to the generated test.  The remaining parameters are then passed into the Table Body function when running the tests.

Under the hood, `DescribeTable` simply generates a new Ginkgo `Describe`.  Each `Entry` is turned into an `It` within the `Describe`.  It's important to understand that the `Describe`s and `It`s are generated at evaluation time (i.e. when Ginkgo constructs the tree of tests and before the tests run).

Individual Entries can be focused (with FEntry) or marked pending (with PEntry or XEntry).  In addition, the entire table can be focused or marked pending with FDescribeTable and PDescribeTable/XDescribeTable.

A description function can be passed to Entry in place of the description. The function is then fed with the entry parameters to genera
*/
func DescribeTable(description string, args ...interface{}) bool {
	generateTable(description, args...)
	return true
}

/*
You can focus a table with `FDescribeTable`.  This is equivalent to `FDescribe`.
*/
func FDescribeTable(description string, args ...interface{}) bool {
	args = append(args, internal.Focus)
	generateTable(description, args...)
	return true
}

/*
You can mark a table as pending with `PDescribeTable`.  This is equivalent to `PDescribe`.
*/
func PDescribeTable(description string, args ...interface{}) bool {
	args = append(args, internal.Pending)
	generateTable(description, args...)
	return true
}

/*
You can mark a table as pending with `XDescribeTable`.  This is equivalent to `XDescribe`.
*/
var XDescribeTable = PDescribeTable

/*
TableEntry represents an entry in a table test.  You generally use the `Entry` constructor.
*/
type TableEntry struct {
	description  interface{}
	decorations  []interface{}
	parameters   []interface{}
	codeLocation types.CodeLocation
}

/*
Entry constructs a TableEntry.

The first argument is a description.  This can be a string, a function that accepts the parameters passed to the TableEntry and returns a string, an EntryDescription format string, or nil.  If nil is provided then the name of the Entry is derived using the table-level entry description.
Subsequent arguments accept any Ginkgo decorators.  These are filtered out and the remaining arguments are passed into the Table Body function associated with the table.

Each Entry ends up generating an individual Ginkgo It.  The body of the it is the Table Body function with the Entry parameters passed in.
*/
func Entry(description interface{}, args ...interface{}) TableEntry {
	decorations, parameters := internal.PartitionDecorations(args...)
	return TableEntry{description: description, decorations: decorations, parameters: parameters, codeLocation: types.NewCodeLocation(1)}
}

/*
You can focus a particular entry with FEntry.  This is equivalent to FIt.
*/
func FEntry(description interface{}, args ...interface{}) TableEntry {
	decorations, parameters := internal.PartitionDecorations(args...)
	decorations = append(decorations, internal.Focus)
	return TableEntry{description: description, decorations: decorations, parameters: parameters, codeLocation: types.NewCodeLocation(1)}
}

/*
You can mark a particular entry as pending with PEntry.  This is equivalent to PIt.
*/
func PEntry(description interface{}, args ...interface{}) TableEntry {
	decorations, parameters := internal.PartitionDecorations(args...)
	decorations = append(decorations, internal.Pending)
	return TableEntry{description: description, decorations: decorations, parameters: parameters, codeLocation: types.NewCodeLocation(1)}
}

/*
You can mark a particular entry as pending with XEntry.  This is equivalent to XIt.
*/
var XEntry = PEntry

func generateTable(description string, args ...interface{}) {
	cl := types.NewCodeLocation(2)
	containerNodeArgs := []interface{}{cl}

	entries := []TableEntry{}
	var itBody interface{}

	var tableLevelEntryDescription interface{}
	tableLevelEntryDescription = func(args ...interface{}) string {
		out := []string{}
		for _, arg := range args {
			out = append(out, fmt.Sprint(arg))
		}
		return "Entry: " + strings.Join(out, ", ")
	}

	for _, arg := range args {
		switch t := reflect.TypeOf(arg); {
		case t == reflect.TypeOf(TableEntry{}):
			entries = append(entries, arg.(TableEntry))
		case t == reflect.TypeOf(EntryDescription("")):
			tableLevelEntryDescription = arg.(EntryDescription).render
		case t.Kind() == reflect.Func && t.NumOut() == 1 && t.Out(0) == reflect.TypeOf(""):
			tableLevelEntryDescription = arg
		case t.Kind() == reflect.Func:
			if itBody != nil {
				exitIfErr(types.GinkgoErrors.MultipleEntryBodyFunctionsForTable(cl))
			}
			itBody = arg
		default:
			containerNodeArgs = append(containerNodeArgs, arg)
		}
	}

	containerNodeArgs = append(containerNodeArgs, func() {
		for _, entry := range entries {
			var err error
			entry := entry
			var description string
			switch t := reflect.TypeOf(entry.description); {
			case t == nil:
				err = validateParameters(tableLevelEntryDescription, entry.parameters, "Entry Description function", entry.codeLocation)
				if err == nil {
					description = invokeFunction(tableLevelEntryDescription, entry.parameters)[0].String()
				}
			case t == reflect.TypeOf(EntryDescription("")):
				description = entry.description.(EntryDescription).render(entry.parameters...)
			case t == reflect.TypeOf(""):
				description = entry.description.(string)
			case t.Kind() == reflect.Func && t.NumOut() == 1 && t.Out(0) == reflect.TypeOf(""):
				err = validateParameters(entry.description, entry.parameters, "Entry Description function", entry.codeLocation)
				if err == nil {
					description = invokeFunction(entry.description, entry.parameters)[0].String()
				}
			default:
				err = types.GinkgoErrors.InvalidEntryDescription(entry.codeLocation)
			}

			if err == nil {
				err = validateParameters(itBody, entry.parameters, "Table Body function", entry.codeLocation)
			}
			itNodeArgs := []interface{}{entry.codeLocation}
			itNodeArgs = append(itNodeArgs, entry.decorations...)
			itNodeArgs = append(itNodeArgs, func() {
				if err != nil {
					panic(err)
				}
				invokeFunction(itBody, entry.parameters)
			})

			pushNode(internal.NewNode(deprecationTracker, types.NodeTypeIt, description, itNodeArgs...))
		}
	})

	pushNode(internal.NewNode(deprecationTracker, types.NodeTypeContainer, description, containerNodeArgs...))
}

func invokeFunction(function interface{}, parameters []interface{}) []reflect.Value {
	inValues := make([]reflect.Value, len(parameters))

	funcType := reflect.TypeOf(function)
	limit := funcType.NumIn()
	if funcType.IsVariadic() {
		limit = limit - 1
	}

	for i := 0; i < limit && i < len(parameters); i++ {
		inValues[i] = computeValue(parameters[i], funcType.In(i))
	}

	if funcType.IsVariadic() {
		variadicType := funcType.In(limit).Elem()
		for i := limit; i < len(parameters); i++ {
			inValues[i] = computeValue(parameters[i], variadicType)
		}
	}

	return reflect.ValueOf(function).Call(inValues)
}

func validateParameters(function interface{}, parameters []interface{}, kind string, cl types.CodeLocation) error {
	funcType := reflect.TypeOf(function)
	limit := funcType.NumIn()
	if funcType.IsVariadic() {
		limit = limit - 1
	}
	if len(parameters) < limit {
		return types.GinkgoErrors.TooFewParametersToTableFunction(limit, len(parameters), kind, cl)
	}
	if len(parameters) > limit && !funcType.IsVariadic() {
		return types.GinkgoErrors.TooManyParametersToTableFunction(limit, len(parameters), kind, cl)
	}
	var i = 0
	for ; i < limit; i++ {
		actual := reflect.TypeOf(parameters[i])
		expected := funcType.In(i)
		if !(actual == nil) && !actual.AssignableTo(expected) {
			return types.GinkgoErrors.IncorrectParameterTypeToTableFunction(i+1, expected, actual, kind, cl)
		}
	}
	if funcType.IsVariadic() {
		expected := funcType.In(limit).Elem()
		for ; i < len(parameters); i++ {
			actual := reflect.TypeOf(parameters[i])
			if !(actual == nil) && !actual.AssignableTo(expected) {
				return types.GinkgoErrors.IncorrectVariadicParameterTypeToTableFunction(expected, actual, kind, cl)
			}
		}
	}

	return nil
}

func computeValue(parameter interface{}, t reflect.Type) reflect.Value {
	if parameter == nil {
		return reflect.Zero(t)
	} else {
		return reflect.ValueOf(parameter)
	}
}