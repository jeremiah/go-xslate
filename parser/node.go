package parser
import (
  "bytes"
  "fmt"
  "reflect"
)

var nodeTextFormat = "%s"

type NodeType int
func (n NodeType) Type() NodeType {
  return n
}

type Node interface {
  Type() NodeType
  String() string
  Copy() Node
  Position() Pos
  Visit(chan Node)
}

type NodeAppender interface {
  Node
  Append(Node)
}

const (
  NodeNoop NodeType = iota
  NodeRoot
  NodeText
  NodeNumber
  NodeInt
  NodeFloat
  NodeIf
  NodeElse
  NodeList
  NodeForeach
  NodeWrapper
  NodeAssignment
  NodeLocalVar
  NodeFetchField
  NodeMethodcall
  NodePrint
  NodePrintRaw
  NodeFetchSymbol
)

func (n NodeType) String() string {
  switch n {
  case NodeNoop:
    return "Noop"
  case NodeRoot:
    return "Root"
  case NodeText:
    return "Text"
  case NodeNumber:
    return "Number"
  case NodeInt:
    return "Int"
  case NodeFloat:
    return "Float"
  case NodeList:
    return "List"
  case NodeForeach:
    return "Foreach"
  case NodeWrapper:
    return "Wrapper"
  case NodeAssignment:
    return "Assignment"
  case NodeLocalVar:
    return "LocalVar"
  case NodeFetchField:
    return "FetchField"
  case NodeMethodcall:
    return "Methodcall"
  case NodePrint:
    return "Print"
  case NodePrintRaw:
    return "PrintRaw"
  case NodeFetchSymbol:
    return "FetchSymbol"
  case NodeIf:
    return "If"
  case NodeElse:
    return "Else"
  default:
    return "Unknown Node"
  }
}

type Pos int

func (p Pos) Position() Pos {
  return p
}

type NoopNode struct {
  NodeType
  Pos
}

type ListNode struct {
  NodeType
  Pos
  Nodes []Node
}

type TextNode struct {
  NodeType
  Pos
  Text []byte
}

type NumberNode struct {
  NodeType
  Pos
  Value reflect.Value
}

func (l *ListNode) Visit(c chan Node) {
  c <- l
  for _, child := range l.Nodes {
    child.Visit(c)
  }
}

func (t *TextNode) Visit(c chan Node) {
  c <- t
}

var noop = &NoopNode {NodeType: NodeNoop}
func NewNoopNode() *NoopNode {
  return noop
}

func (n NoopNode) Copy() Node {
  return noop
}

func (n *NoopNode) String() string {
  return "noop"
}

func (n *NoopNode) Visit(chan Node) {
  // ignore
}

func NewListNode(pos Pos) *ListNode {
  return &ListNode {NodeType: NodeList, Pos: pos, Nodes: []Node {}}
}

func (l *ListNode) Copy() Node {
  n := NewListNode(l.Pos)
  n.Nodes = make([]Node, len(l.Nodes))
  copy(n.Nodes, l.Nodes)
  return n
}

func (l *ListNode) String() string {
  return l.NodeType.String()
}

func (l *ListNode) Append(n Node) {
  l.Nodes = append(l.Nodes, n)
}

func NewTextNode(pos Pos, arg string) *TextNode {
  return &TextNode {NodeType: NodeText, Pos: pos, Text: []byte(arg)}
}

func (n *TextNode) Copy() Node {
  return NewTextNode(n.Pos, string(n.Text))
}

func (n *TextNode) String() string {
  return fmt.Sprintf("%s %s", n.NodeType, n.Text)
}

func NewWrapperNode(pos Pos, template string) *ListNode {
  n := NewListNode(pos)
  n.NodeType = NodeWrapper
  n.Append(NewTextNode(pos, template))
  return n
}

type AssignmentNode struct {
  NodeType
  Pos
  Assignee *LocalVarNode
  Expression Node
}

func NewAssignmentNode(pos Pos, symbol string) *AssignmentNode {
  n := &AssignmentNode {
    NodeAssignment,
    pos,
    NewLocalVarNode(pos, symbol, 0), // TODO
    nil,
  }
  return n
}

func (n *AssignmentNode) Copy() Node {
  x := &AssignmentNode {
    NodeAssignment,
    n.Pos,
    n.Assignee,
    n.Expression,
  }
  return x
}

func (n *AssignmentNode) Visit(c chan Node) {
  c <- n
  c <- n.Assignee
  c <- n.Expression
}

func (n *AssignmentNode) String() string {
  return n.NodeType.String()
}

type LocalVarNode struct {
  NodeType
  Pos
  Name string
  Offset int
}

func NewLocalVarNode(pos Pos, symbol string, idx int) *LocalVarNode {
  n := &LocalVarNode {
    NodeLocalVar,
    pos,
    symbol,
    idx,
  }
  return n
}

func (n *LocalVarNode) Copy() Node {
  return NewLocalVarNode(n.Pos, n.Name, n.Offset)
}

func (n *LocalVarNode) Visit(c chan Node) {
  c <- n
}

func (n *LocalVarNode) String() string {
  return fmt.Sprintf("%s %s (%d)", n.NodeType, n.Name, n.Offset)
}

type ForeachNode struct {
  *ListNode
  IndexVarName  string
  IndexVarIdx   int
  List          Node
}

func NewForeachNode(pos Pos, symbol string) *ForeachNode {
  n := &ForeachNode {
    ListNode: NewListNode(pos),
    IndexVarName: symbol,
    IndexVarIdx: 0,
    List: nil,
  }
  n.NodeType = NodeForeach
  return n
}

func (n *ForeachNode) Visit(c chan Node) {
  c <- n
  // Skip the list node that we contain
  for _, child := range n.ListNode.Nodes {
    child.Visit(c)
  }
}

func (n *ForeachNode) Copy() Node {
  x := &ForeachNode {
    ListNode: NewListNode(n.Pos),
    IndexVarName: n.IndexVarName,
    IndexVarIdx: n.IndexVarIdx,
    List: n.List.Copy(),
  }
  x.NodeType = NodeForeach
  return n
}

func (n *ForeachNode) String() string {
  b := &bytes.Buffer {}
  fmt.Fprintf(b, "%s %s (%d)", n.NodeType, n.IndexVarName, n.IndexVarIdx)
  return b.String()
}

func NewMethodcallNode(pos Pos, invocant, method string, args Node) *ListNode {
  n := NewListNode(pos)
  n.NodeType = NodeMethodcall
  n.Append(NewLocalVarNode(pos, invocant, 0)) // TODO
  n.Append(NewTextNode(pos, method))
  n.Append(args)
  return n
}

type FetchFieldNode struct {
  NodeType
  Pos
  Container Node
  FieldName string
}

func NewFetchFieldNode(pos Pos, container Node, field string) *FetchFieldNode {
  n := &FetchFieldNode {
    NodeFetchField,
    pos,
    container,
    field,
  }
  return n
}

func (n *FetchFieldNode) Copy() Node {
  return &FetchFieldNode {
    NodeFetchField,
    n.Pos,
    n.Container.Copy(),
    n.FieldName,
  }
}

func (n *FetchFieldNode) Visit(c chan Node) {
  c <- n
  n.Container.Visit(c)
}

func NewRootNode() *ListNode {
  n := NewListNode(0)
  n.NodeType = NodeRoot
  return n
}

func NewNumberNode(pos Pos, num reflect.Value) *NumberNode {
  return &NumberNode {NodeType: NodeNumber, Pos: pos, Value: num}
}

func (n *NumberNode) Copy() Node {
  x := NewNumberNode(n.Pos, n.Value)
  x.NodeType = n.NodeType
  return x
}

func (n *NumberNode) Visit(c chan Node) {
  c <- n
}

func NewIntNode(pos Pos, v int64) *NumberNode {
  n := NewNumberNode(pos, reflect.ValueOf(v))
  n.NodeType = NodeInt
  return n
}

func NewFloatNode(pos Pos, v float64) *NumberNode {
  n := NewNumberNode(pos, reflect.ValueOf(v))
  n.NodeType = NodeFloat
  return n
}

func NewPrintNode(pos Pos, arg Node) *ListNode {
  n := NewListNode(pos)
  n.NodeType = NodePrint
  n.Append(arg)
  return n
}

func NewPrintRawNode(pos Pos) *ListNode {
  n := NewListNode(pos)
  n.NodeType = NodePrintRaw
  return n
}

func NewFetchSymbolNode(pos Pos, symbol string) *TextNode {
  n := NewTextNode(pos, symbol)
  n.NodeType = NodeFetchSymbol
  return n
}

type IfNode struct {
  *ListNode
  BooleanExpression Node
}

func NewIfNode(pos Pos, exp Node) *IfNode {
  n := &IfNode {
    NewListNode(pos),
    exp,
  }
  n.NodeType = NodeIf
  return n
}

func (n *IfNode) Copy() Node {
  x := &IfNode {
    n.ListNode.Copy().(*ListNode),
    nil,
  }
  if e := n.BooleanExpression; e != nil {
    x.BooleanExpression = e.Copy()
  }

  x.ListNode = n.ListNode.Copy().(*ListNode)

  return x
}

func (n *IfNode) Visit(c chan Node) {
  c <- n
  c <- n.BooleanExpression
  for _, child := range n.ListNode.Nodes {
    c <- child
  }
}

type ElseNode struct {
  *ListNode
  IfNode Node
}

func NewElseNode(pos Pos) *ElseNode {
  n := &ElseNode {
    NewListNode(pos),
    nil,
  }
  n.NodeType = NodeElse
  return n
}
