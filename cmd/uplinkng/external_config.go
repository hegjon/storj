// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"

	"github.com/zeebo/errs"
	"github.com/zeebo/ini"
)

// loadConfig loads the configuration file from disk if it is not already loaded.
// This makes calls to loadConfig idempotent.
func (ex *external) loadConfig() error {
	if ex.config.values != nil {
		return nil
	}
	ex.config.values = make(map[string][]string)

	fh, err := os.Open(ex.ConfigFile())
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = fh.Close() }()

	err = ini.Read(fh, func(ent ini.Entry) error {
		if ent.Section != "" {
			ent.Key = ent.Section + "." + ent.Key
		}
		ex.config.values[ent.Key] = append(ex.config.values[ent.Key], ent.Value)
		return nil
	})
	if err != nil {
		return err
	}

	ex.config.loaded = true
	return nil
}

// saveConfig writes out the config file using the provided values.
// It is only intended to be used during initial migration and setup.
func (ex *external) saveConfig(entries []ini.Entry) error {
	// TODO(jeff): write it atomically

	newFh, err := os.Create(ex.ConfigFile())
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = newFh.Close() }()

	err = ini.Write(newFh, func(emit func(ini.Entry)) {
		for _, ent := range entries {
			emit(ent)
		}
	})
	if err != nil {
		return errs.Wrap(err)
	}

	if err := newFh.Sync(); err != nil {
		return errs.Wrap(err)
	}

	if err := newFh.Close(); err != nil {
		return errs.Wrap(err)
	}

	return nil
}
