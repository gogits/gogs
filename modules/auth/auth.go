// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/codegangsta/martini"

	"github.com/gogits/binding"

	"github.com/gogits/gogs/modules/base"
	"github.com/gogits/gogs/modules/log"
)

// Web form interface.
type Form interface {
	Name(field string) string
}

type RegisterForm struct {
	UserName     string `form:"username" binding:"Required;AlphaDash;MaxSize(30)"`
	Email        string `form:"email" binding:"Required;Email;MaxSize(50)"`
	Password     string `form:"passwd" binding:"Required;MinSize(6);MaxSize(30)"`
	RetypePasswd string `form:"retypepasswd"`
}

func (f *RegisterForm) Name(field string) string {
	names := map[string]string{
		"UserName":     "Username",
		"Email":        "E-mail address",
		"Password":     "Password",
		"RetypePasswd": "Re-type password",
	}
	return names[field]
}

func (f *RegisterForm) Validate(errors *binding.Errors, req *http.Request, context martini.Context) {
	if req.Method == "GET" || errors.Count() == 0 {
		return
	}

	data := context.Get(reflect.TypeOf(base.TmplData{})).Interface().(base.TmplData)
	data["HasError"] = true
	AssignForm(f, data)

	if len(errors.Overall) > 0 {
		for _, err := range errors.Overall {
			log.Error("RegisterForm.Validate: %v", err)
		}
		return
	}

	validate(errors, data, f)
}

type LogInForm struct {
	UserName string `form:"username" binding:"Required;AlphaDash;MaxSize(30)"`
	Password string `form:"passwd" binding:"Required;MinSize(6);MaxSize(30)"`
}

func (f *LogInForm) Name(field string) string {
	names := map[string]string{
		"UserName": "Username",
		"Password": "Password",
	}
	return names[field]
}

func (f *LogInForm) Validate(errors *binding.Errors, req *http.Request, context martini.Context) {
	if req.Method == "GET" || errors.Count() == 0 {
		return
	}

	data := context.Get(reflect.TypeOf(base.TmplData{})).Interface().(base.TmplData)
	data["HasError"] = true
	AssignForm(f, data)

	if len(errors.Overall) > 0 {
		for _, err := range errors.Overall {
			log.Error("LogInForm.Validate: %v", err)
		}
		return
	}

	validate(errors, data, f)
}

func getMinMaxSize(field reflect.StructField) string {
	for _, rule := range strings.Split(field.Tag.Get("binding"), ";") {
		if strings.HasPrefix(rule, "MinSize(") || strings.HasPrefix(rule, "MaxSize(") {
			return rule[8 : len(rule)-1]
		}
	}
	return ""
}

func validate(errors *binding.Errors, data base.TmplData, form Form) {
	typ := reflect.TypeOf(form)
	val := reflect.ValueOf(form)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		fieldName := field.Tag.Get("form")
		// Allow ignored fields in the struct
		if fieldName == "-" {
			continue
		}

		if err, ok := errors.Fields[field.Name]; ok {
			data["Err_"+field.Name] = true
			switch err {
			case binding.RequireError:
				data["ErrorMsg"] = form.Name(field.Name) + " cannot be empty"
			case binding.AlphaDashError:
				data["ErrorMsg"] = form.Name(field.Name) + " must be valid alpha or numeric or dash(-_) characters"
			case binding.MinSizeError:
				data["ErrorMsg"] = form.Name(field.Name) + " must contain at least " + getMinMaxSize(field) + " characters"
			case binding.MaxSizeError:
				data["ErrorMsg"] = form.Name(field.Name) + " must contain at most " + getMinMaxSize(field) + " characters"
			case binding.EmailError:
				data["ErrorMsg"] = form.Name(field.Name) + " is not valid"
			default:
				data["ErrorMsg"] = "Unknown error: " + err
			}
			return
		}
	}
}

// AssignForm assign form values back to the template data.
func AssignForm(form interface{}, data base.TmplData) {
	typ := reflect.TypeOf(form)
	val := reflect.ValueOf(form)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		fieldName := field.Tag.Get("form")
		// Allow ignored fields in the struct
		if fieldName == "-" {
			continue
		}

		data[fieldName] = val.Field(i).Interface()
	}
}
