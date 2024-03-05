package parser

import (
	"go/ast"
	t "static_analyser/pkg/types"
	"static_analyser/pkg/util"
	"strings"
)

func FindServiceDiscoveryWrappers(node ast.Node) []t.ServiceDiscoveryWrapper {
	// Different nacos SDK functions for service discovery
	select_sdk := []string{"GetService", "SelectAllInstances", "SelectOneHealthyInstance", "SelectInstances", "Subscribe"}
	select_params := []string{"GetServiceParam", "SelectAllInstancesParam", "SelectOneHealthyInstanceParam", "SelectInstancesParam", "SubscribeParam"}

	var paramNames = []string{}
	var wrapper string
	var instances []t.ServiceDiscoveryWrapper
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		switch n := n.(type) {
		case *ast.FuncDecl:
			// Function declaration
			wrapper = n.Name.Name
			for _, param := range n.Type.Params.List {
				for _, name := range param.Names {
					paramNames = append(paramNames, name.Name)
				}
			}
			// log.Printf("Parameter names: %v\n", paramNames)

		case *ast.CallExpr:
			// log.Printf("Call expression: %s\n", n.Fun)
			if selExpr, ok := n.Fun.(*ast.SelectorExpr); ok {
				// If the function is a list of nacos sdk functions
				if util.Contains(select_sdk, selExpr.Sel.Name) {
					// log.Printf("%s ", selExpr.Sel.Name)
					for _, arg := range n.Args {

						switch arg := arg.(type) {
						case *ast.CompositeLit:
							if sel, ok := arg.Type.(*ast.SelectorExpr); ok {

								if util.Contains(select_params, sel.Sel.Name) {

									instance := t.ServiceDiscoveryWrapper{}
									instance.Wrapper = wrapper
									for _, elt := range arg.Elts {
										if kv, ok := elt.(*ast.KeyValueExpr); ok {
											if key, ok := kv.Key.(*ast.Ident); ok {
												if key.Name == "ServiceName" {
													switch v := kv.Value.(type) {
													case *ast.Ident:
														for i, paramName := range paramNames {
															if paramName == strings.TrimSpace(v.Name) {
																instance.ServiceName = t.WrapperParams{Position: i}
															}
															if instance.ServiceName == nil {
																instance.ServiceName = util.FindConstValue(node, strings.TrimSpace(v.Name), wrapper)
															}
														}
													case *ast.BasicLit:
														instance.ServiceName = strings.ReplaceAll(strings.TrimSpace(v.Value), "\"", "")
													}
												}
											}
										}
									}

									instances = append(instances, instance)

								}
							}
						}
					}
				}
			}
		}
		return true
	})
	return instances
}