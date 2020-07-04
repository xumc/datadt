package display

type ChangeItem struct {
	TableName string
	Action    string
	Changes   [][3]string
}

type Outputer interface {
	Inputer() chan <- interface{}
	Run() error
}
