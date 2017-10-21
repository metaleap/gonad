package main

import (
	"fmt"
	"strings"

	"github.com/metaleap/go-util-str"
)

/*
Represents everything in coreimp.json files
generated by purs with the --dump-coreimp option.

In here we mostly deal with the stuff in Body
(the actual JS-like core-imperative AST ie funcs + vars),
whereas DeclAnns and DeclEnv (PureScript types &
signatures) are mostly handled in ps-coreimp-decls.go.
*/

var (
	strReplUnprime = strings.NewReplacer("$prime", "'")
)

type coreImp struct { // we skip unmarshaling what isn't used for now, but DO keep these around commented-out:
	// BuiltWith  string            `json:"builtWith"`
	// ModuleName string            `json:"moduleName"`
	// ModulePath string            `json:"modulePath"`
	// Comments   []*coreImpComment `json:"comments"`
	// Foreign    []string          `json:"foreign"`
	// Exports    []string          `json:"exports"`
	Imps     [][]string     `json:"imports"`
	Body     coreImpAsts    `json:"body"`
	DeclAnns []*coreImpDecl `json:"declAnns"`
	DeclEnv  coreImpEnv     `json:"declEnv"`

	namedRequires map[string]string
	mod           *modPkg
}

func (me *coreImp) prep() {
	for _, da := range me.DeclAnns {
		da.prep()
	}
	me.DeclEnv.prep()
}

type coreImpComment struct {
	LineComment  string
	BlockComment string
}

type coreImpDecl struct {
	BindType string           `json:"bindType"`
	Ident    string           `json:"identifier"`
	Ann      *coreImpDeclAnn  `json:"annotation"`
	Expr     *coreImpDeclExpr `json:"expression"`
}

func (me *coreImpDecl) prep() {
	if me.Ann != nil {
		me.Ann.prep()
	}
	if me.Expr != nil {
		me.Expr.prep()
	}
}

type coreImpDeclAnn struct {
	SourceSpan *coreImpSourceSpan `json:"sourceSpan"`
	Type       *coreImpEnvTagType `json:"type"`
	Comments   []*coreImpComment  `json:"comments"`
	Meta       struct {
		MetaType   string   `json:"metaType"`        // IsConstructor or IsNewtype or IsTypeClassConstructor or IsForeign
		CtorType   string   `json:"constructorType"` // if MetaType=IsConstructor: SumType or ProductType
		CtorIdents []string `json:"identifiers"`     // if MetaType=IsConstructor
	} `json:"meta"`
}

func (me *coreImpDeclAnn) prep() {
	if me.Type != nil {
		me.Type.prep()
	}
}

type coreImpDeclExpr struct {
	Ann        *coreImpDeclAnn `json:"annotation"`
	ExprTag    string          `json:"type"`            // Var or Literal or Abs or App or Let or Constructor (less likely: or Accessor or ObjectUpdate or Case)
	CtorName   string          `json:"constructorName"` // if ExprTag=Constructor
	CtorType   string          `json:"typeName"`        // if ExprTag=Constructor
	CtorFields []string        `json:"fieldNames"`      // if ExprTag=Constructor
}

func (me *coreImpDeclExpr) prep() {
	if me.Ann != nil {
		me.Ann.prep()
	}
}

type coreImpSourceSpan struct {
	Name  string `json:"name"`
	Start []int  `json:"start"`
	End   []int  `json:"end"`
}

type coreImpAsts []*coreImpAst

type coreImpAst struct {
	AstSourceSpan  *coreImpSourceSpan `json:"sourceSpan"`
	AstTag         string             `json:"tag"`
	AstBody        *coreImpAst        `json:"body"`
	AstRight       *coreImpAst        `json:"rhs"`
	AstCommentDecl *coreImpAst        `json:"decl"`
	AstApplArgs    coreImpAsts        `json:"args"`
	AstOp          string             `json:"op"`
	AstFuncParams  []string           `json:"params"`
	AstFor1        *coreImpAst        `json:"for1"`
	AstFor2        *coreImpAst        `json:"for2"`
	AstThen        *coreImpAst        `json:"then"`
	AstElse        *coreImpAst        `json:"else"`

	Function               string
	StringLiteral          string
	BooleanLiteral         bool
	NumericLiteral_Integer int
	NumericLiteral_Double  float64
	Block                  coreImpAsts
	Var                    string
	VariableIntroduction   string
	While                  *coreImpAst
	App                    *coreImpAst
	Unary                  *coreImpAst
	Comment                []*coreImpComment
	Binary                 *coreImpAst
	ForIn                  string
	For                    string
	IfElse                 *coreImpAst
	ObjectLiteral          []map[string]*coreImpAst
	Return                 *coreImpAst
	Throw                  *coreImpAst
	ArrayLiteral           coreImpAsts
	Assignment             *coreImpAst
	Indexer                *coreImpAst
	Accessor               *coreImpAst
	InstanceOf             *coreImpAst

	parent *coreImpAst
	root   *coreImp
}

func (me *coreImpAst) astForceIntoBlock(into *gIrABlock) {
	switch maybebody := me.ciAstToGIrAst().(type) {
	case *gIrABlock:
		into.Body = maybebody.Body
		for _, a := range into.Body {
			a.Base().parent = into
		}
	default:
		into.Add(maybebody)
	}
}

func (me *coreImpAst) ciAstToGIrAst() (a gIrA) {
	istopleveldecl := (me.parent == nil)
	switch me.AstTag {
	case "StringLiteral":
		a = ªS(me.StringLiteral)
	case "BooleanLiteral":
		a = ªB(me.BooleanLiteral)
	case "NumericLiteral_Double":
		a = ªF(me.NumericLiteral_Double)
	case "NumericLiteral_Integer":
		a = ªI(me.NumericLiteral_Integer)
	case "Var":
		v := ªSymPs(me.Var, me.root.mod.girMeta.hasExport(me.Var))
		a = v
	case "Block":
		b := ªBlock()
		for _, c := range me.Block {
			b.Add(c.ciAstToGIrAst())
		}
		a = b
	case "While":
		f := ªFor()
		f.ForCond = me.While.ciAstToGIrAst()
		f.ForCond.Base().parent = f
		me.AstBody.astForceIntoBlock(f.ForDo)
		a = f
	case "ForIn":
		f := ªFor()
		f.ForRange = ªLet("", me.ForIn, me.AstFor1.ciAstToGIrAst())
		f.ForRange.parent = f
		me.AstBody.astForceIntoBlock(f.ForDo)
		a = f
	case "For":
		f := ªFor()
		fs := ªSymPs(me.For, me.root.mod.girMeta.hasExport(me.For))
		f.ForInit = []*gIrALet{ªLet("", me.For, me.AstFor1.ciAstToGIrAst())}
		f.ForInit[0].parent = f
		fscmp, fsset, fsadd := *fs, *fs, *fs // quirky that we need these copies but we do
		f.ForCond = ªO2(&fscmp, "<", me.AstFor2.ciAstToGIrAst())
		f.ForCond.Base().parent = f
		f.ForStep = []*gIrASet{ªSet(&fsset, ªO2(&fsadd, "+", ªI(1)))}
		f.ForStep[0].parent = f
		me.AstBody.astForceIntoBlock(f.ForDo)
		a = f
	case "IfElse":
		i := ªIf(me.IfElse.ciAstToGIrAst())
		me.AstThen.astForceIntoBlock(i.Then)
		if me.AstElse != nil {
			i.Else = ªBlock()
			me.AstElse.astForceIntoBlock(i.Else)
			i.Else.parent = i
		}
		a = i
	case "App":
		c := ªCall(me.App.ciAstToGIrAst())
		for _, carg := range me.AstApplArgs {
			arg := carg.ciAstToGIrAst()
			arg.Base().parent = c
			c.CallArgs = append(c.CallArgs, arg)
		}
		a = c
	case "VariableIntroduction":
		v := ªLet("", me.VariableIntroduction, nil)
		if istopleveldecl && ustr.BeginsUpper(me.VariableIntroduction) {
			v.WasTypeFunc = true
		}
		if me.AstRight != nil {
			v.LetVal = me.AstRight.ciAstToGIrAst()
			v.LetVal.Base().parent = v
		}
		a = v
	case "Function":
		f := ªFunc()
		if istopleveldecl && len(me.Function) > 0 && ustr.BeginsUpper(me.Function) {
			f.WasTypeFunc = true
		}
		f.setBothNamesFromPsName(me.Function)
		f.RefFunc = &gIrATypeRefFunc{}
		for _, fpn := range me.AstFuncParams {
			arg := &gIrANamedTypeRef{}
			arg.setBothNamesFromPsName(fpn)
			f.RefFunc.Args = append(f.RefFunc.Args, arg)
		}
		me.AstBody.astForceIntoBlock(f.FuncImpl)
		f.method.body = f.FuncImpl
		a = f
	case "Unary":
		o := ªO1(me.AstOp, me.Unary.ciAstToGIrAst())
		switch o.Op1 {
		case "Negate":
			o.Op1 = "-"
		case "Not":
			o.Op1 = "!"
		case "Positive":
			o.Op1 = "+"
		case "BitwiseNot":
			o.Op1 = "^"
		case "New":
			o.Op1 = "&"
		default:
			panic("unrecognized unary op '" + o.Op1 + "', please report!")
			o.Op1 = "?" + o.Op1 + "?"
		}
		a = o
	case "Binary":
		o := ªO2(me.Binary.ciAstToGIrAst(), me.AstOp, me.AstRight.ciAstToGIrAst())
		switch o.Op2 {
		case "Add":
			o.Op2 = "+"
		case "Subtract":
			o.Op2 = "-"
		case "Multiply":
			o.Op2 = "*"
		case "Divide":
			o.Op2 = "/"
		case "Modulus":
			o.Op2 = "%"
		case "EqualTo":
			o.Op2 = "=="
		case "NotEqualTo":
			o.Op2 = "!="
		case "LessThan":
			o.Op2 = "<"
		case "LessThanOrEqualTo":
			o.Op2 = "<="
		case "GreaterThan":
			o.Op2 = ">"
		case "GreaterThanOrEqualTo":
			o.Op2 = ">="
		case "And":
			o.Op2 = "&&"
		case "Or":
			o.Op2 = "||"
		case "BitwiseAnd":
			o.Op2 = "&"
		case "BitwiseOr":
			o.Op2 = "|"
		case "BitwiseXor":
			o.Op2 = "^"
		case "ShiftLeft":
			o.Op2 = "<<"
		case "ShiftRight":
			o.Op2 = ">>"
		case "ZeroFillShiftRight":
			o.Op2 = "&^"
		default:
			o.Op2 = "?" + o.Op2 + "?"
			panic("unrecognized binary op '" + o.Op2 + "', please report!")
		}
		a = o
	case "Comment":
		c := ªComments(me.Comment...)
		a = c
	case "ObjectLiteral":
		o := ªO(nil)
		for _, namevaluepair := range me.ObjectLiteral {
			for onekey, oneval := range namevaluepair {
				ofv := ªOFld(oneval.ciAstToGIrAst())
				ofv.setBothNamesFromPsName(onekey)
				ofv.parent = o
				o.ObjFields = append(o.ObjFields, ofv)
				break
			}
		}
		a = o
	case "ReturnNoResult":
		r := ªRet(nil)
		a = r
	case "Return":
		r := ªRet(me.Return.ciAstToGIrAst())
		a = r
	case "Throw":
		r := ªPanic(me.Throw.ciAstToGIrAst())
		a = r
	case "ArrayLiteral":
		exprs := make([]gIrA, 0, len(me.ArrayLiteral))
		for _, v := range me.ArrayLiteral {
			exprs = append(exprs, v.ciAstToGIrAst())
		}
		l := ªA(exprs...)
		a = l
	case "Assignment":
		o := ªSet(me.Assignment.ciAstToGIrAst(), me.AstRight.ciAstToGIrAst())
		a = o
	case "Indexer":
		if me.AstRight.AstTag == "StringLiteral" { // TODO will need to differentiate better between a real property or an obj-dict-key
			dv := ªSymPs(me.AstRight.StringLiteral, me.root.mod.girMeta.hasExport(me.AstRight.StringLiteral))
			a = ªDot(me.Indexer.ciAstToGIrAst(), dv)
		} else {
			a = ªIndex(me.Indexer.ciAstToGIrAst(), me.AstRight.ciAstToGIrAst())
		}
	case "InstanceOf":
		if len(me.AstRight.Var) > 0 {
			a = ªIs(me.InstanceOf.ciAstToGIrAst(), me.AstRight.Var)
		} else /*if me.AstRight.Indexer != nil*/ {
			adot := me.AstRight.ciAstToGIrAst().(*gIrADot)
			a = ªIs(me.InstanceOf.ciAstToGIrAst(), findModuleByPName(adot.DotLeft.(*gIrASym).NamePs).qName+"."+adot.DotRight.(*gIrASym).NamePs)
		}
	default:
		panic(fmt.Errorf("Just below %v: unrecognized coreImp AST-tag, please report: %s", me.parent, me.AstTag))
	}
	if ab := a.Base(); ab != nil {
		ab.Comments = me.Comment
	}
	return
}

func (me *coreImp) preProcessTopLevel() error {
	me.namedRequires = map[string]string{}
	me.Body = me.preProcessAsts(nil, me.Body...)
	i := 0
	ditch := func() {
		me.Body = append(me.Body[:i], me.Body[i+1:]...)
		i -= 1
	}
	for i = 0; i < len(me.Body); i++ {
		a := me.Body[i]
		if a.StringLiteral == "use strict" {
			//	"use strict"
			ditch()
		} else if a.Assignment != nil && a.Assignment.Indexer != nil && a.Assignment.Indexer.Var == "module" && a.Assignment.AstRight != nil && a.Assignment.AstRight.StringLiteral == "exports" {
			//	module.exports = ..
			ditch()
		} else if a.AstTag == "VariableIntroduction" {
			if a.AstRight != nil && a.AstRight.App != nil && a.AstRight.App.Var == "require" && len(a.AstRight.AstApplArgs) == 1 && len(a.AstRight.AstApplArgs[0].StringLiteral) > 0 {
				me.namedRequires[a.VariableIntroduction] = a.AstRight.AstApplArgs[0].StringLiteral
				ditch()
			} else if a.AstRight != nil && a.AstRight.AstTag == "Function" {
				// turn top-level `var foo = func()` into `func foo()`
				a.AstRight.Function = a.VariableIntroduction
				a = a.AstRight
				a.parent, me.Body[i] = nil, a
			}
		} else if a.AstTag != "Function" && a.AstTag != "VariableIntroduction" && a.AstTag != "Comment" {
			return fmt.Errorf("Encountered unexpected top-level AST tag, please report: %s", a.AstTag)
		}
	}
	return nil
}

func (me *coreImp) preProcessAsts(parent *coreImpAst, asts ...*coreImpAst) coreImpAsts {
	if parent != nil {
		parent.root = me
	}
	for i := 0; i < len(asts); i++ {
		if cia := asts[i]; cia != nil && cia.AstTag == "Comment" && cia.AstCommentDecl != nil {
			if cia.AstCommentDecl.AstTag == "Comment" {
				panic("Please report: encountered comments nesting.")
			}
			cdecl := cia.AstCommentDecl
			cia.AstCommentDecl = nil
			cdecl.Comment = cia.Comment
			asts[i] = cdecl
			i--
		}
	}
	for _, a := range asts {
		if a != nil {
			for _, sym := range []*string{&a.For, &a.ForIn, &a.Function, &a.Var, &a.VariableIntroduction} {
				if len(*sym) > 0 {
					*sym = strReplUnprime.Replace(*sym)
				}
			}
			for i, mkv := range a.ObjectLiteral {
				for onename, oneval := range mkv {
					if nuname := strReplUnprime.Replace(onename); nuname != onename {
						mkv = map[string]*coreImpAst{}
						mkv[nuname] = oneval
						a.ObjectLiteral[i] = mkv
					}
				}
			}
			for i, afp := range a.AstFuncParams {
				a.AstFuncParams[i] = strReplUnprime.Replace(afp)
			}

			a.root = me
			a.parent = parent
			a.App = me.preProcessAsts(a, a.App)[0]
			a.ArrayLiteral = me.preProcessAsts(a, a.ArrayLiteral...)
			a.Assignment = me.preProcessAsts(a, a.Assignment)[0]
			a.AstApplArgs = me.preProcessAsts(a, a.AstApplArgs...)
			a.AstBody = me.preProcessAsts(a, a.AstBody)[0]
			a.AstCommentDecl = me.preProcessAsts(a, a.AstCommentDecl)[0]
			a.AstFor1 = me.preProcessAsts(a, a.AstFor1)[0]
			a.AstFor2 = me.preProcessAsts(a, a.AstFor2)[0]
			a.AstElse = me.preProcessAsts(a, a.AstElse)[0]
			a.AstThen = me.preProcessAsts(a, a.AstThen)[0]
			a.AstRight = me.preProcessAsts(a, a.AstRight)[0]
			a.Binary = me.preProcessAsts(a, a.Binary)[0]
			a.Block = me.preProcessAsts(a, a.Block...)
			a.IfElse = me.preProcessAsts(a, a.IfElse)[0]
			a.Indexer = me.preProcessAsts(a, a.Indexer)[0]
			a.Assignment = me.preProcessAsts(a, a.Assignment)[0]
			a.InstanceOf = me.preProcessAsts(a, a.InstanceOf)[0]
			a.Return = me.preProcessAsts(a, a.Return)[0]
			a.Throw = me.preProcessAsts(a, a.Throw)[0]
			a.Unary = me.preProcessAsts(a, a.Unary)[0]
			a.While = me.preProcessAsts(a, a.While)[0]
			for km, m := range a.ObjectLiteral {
				for kx, expr := range m {
					m[kx] = me.preProcessAsts(a, expr)[0]
				}
				a.ObjectLiteral[km] = m
			}
		}
	}
	return asts
}
