package creative

var AI AIrunner

type AIrunner interface {
	Run(request string) (responce string, err error)
}
