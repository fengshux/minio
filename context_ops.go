package main

import (
	"errors"
	"fmt"

	"s3m/commands"
)

type contextOps struct {
	configPath string
}

func (o *contextOps) ConfigPath() string {
	if o.configPath == "" {
		o.configPath = GetConfigPath()
	}
	return o.configPath
}

func (o *contextOps) loadStore() (*ContextStore, error) {
	return parseContextStore(o.ConfigPath())
}

func (o *contextOps) List() ([]commands.ContextInfo, error) {
	store, err := o.loadStore()
	if err != nil {
		return nil, err
	}
	out := make([]commands.ContextInfo, 0, len(store.Contexts))
	for name, ctx := range store.Contexts {
		out = append(out, commands.ContextInfo{
			Name:      name,
			Endpoint:  ctx.Endpoint,
			UseSSL:    ctx.UseSSL,
			IsCurrent: name == store.Current,
		})
	}
	return out, nil
}

func (o *contextOps) CurrentName() (string, error) {
	store, err := o.loadStore()
	if err != nil {
		return "", err
	}
	if store.Current == "" {
		return "", errors.New("未设置 current-context，请使用 's3m context set <name>' 创建")
	}
	return store.Current, nil
}

func (o *contextOps) SetCurrent(name string) error {
	store, err := o.loadStore()
	if err != nil {
		return err
	}
	if _, ok := store.Contexts[name]; !ok {
		return fmt.Errorf("context %q 不存在", name)
	}
	store.Current = name
	return SaveContextStore(o.ConfigPath(), store)
}

func (o *contextOps) Show(name string) (string, string, commands.ContextInfo, error) {
	store, err := o.loadStore()
	if err != nil {
		return "", "", commands.ContextInfo{}, err
	}
	if name == "" {
		name = store.Current
	}
	if name == "" {
		return "", "", commands.ContextInfo{}, errors.New("未指定 context，也未设置 current-context")
	}
	ctx, ok := store.Contexts[name]
	if !ok {
		return "", "", commands.ContextInfo{}, fmt.Errorf("context %q 不存在", name)
	}
	return ctx.AccessKey, ctx.SecretKey, commands.ContextInfo{
		Name:      name,
		Endpoint:  ctx.Endpoint,
		UseSSL:    ctx.UseSSL,
		IsCurrent: name == store.Current,
	}, nil
}

func (o *contextOps) Upsert(name, endpoint string, useSSL bool, accessKey, secretKey string) error {
	store, err := o.loadStore()
	if err != nil {
		store = &ContextStore{Contexts: map[string]Context{}}
	}
	store.Contexts[name] = Context{
		Endpoint:  endpoint,
		UseSSL:    useSSL,
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
	if store.Current == "" {
		store.Current = name
	}
	return SaveContextStore(o.ConfigPath(), store)
}

func (o *contextOps) Rename(oldName, newName string) error {
	if oldName == newName {
		return nil
	}
	store, err := o.loadStore()
	if err != nil {
		return err
	}
	ctx, ok := store.Contexts[oldName]
	if !ok {
		return fmt.Errorf("context %q 不存在", oldName)
	}
	if _, dup := store.Contexts[newName]; dup {
		return fmt.Errorf("context %q 已存在", newName)
	}
	delete(store.Contexts, oldName)
	store.Contexts[newName] = ctx
	if store.Current == oldName {
		store.Current = newName
	}
	return SaveContextStore(o.ConfigPath(), store)
}

func (o *contextOps) Delete(name string) error {
	store, err := o.loadStore()
	if err != nil {
		return err
	}
	if _, ok := store.Contexts[name]; !ok {
		return fmt.Errorf("context %q 不存在", name)
	}
	delete(store.Contexts, name)
	if store.Current == name {
		store.Current = ""
	}
	return SaveContextStore(o.ConfigPath(), store)
}
