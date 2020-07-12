package test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	au "github.com/tsiemens/gmail-tools/aliasutil"
)

func TestClassifyArgs(t *testing.T) {
	args := au.ClassifyArgs([]string{"foo", "bar"})
	assert.Equal(t, args.PosArgs, []string{"foo", "bar"})

	args = au.ClassifyArgs([]string{"--", "foo", "bar"})
	assert.Equal(t, args.PosArgs, []string{"--", "foo", "bar"})

	args = au.ClassifyArgs([]string{"foo", "--", "bar"})
	assert.Equal(t, args.UnknownArgs, []string{})
	assert.Equal(t, args.PosArgs, []string{"foo", "--", "bar"})

	args = au.ClassifyArgs([]string{"foo", "bar", "--"})
	assert.Equal(t, args.UnknownArgs, []string{})
	assert.Equal(t, args.PosArgs, []string{"foo", "bar", "--"})

	args = au.ClassifyArgs([]string{"foo", "bar", "--", "--"})
	assert.Equal(t, args.UnknownArgs, []string{})
	assert.Equal(t, args.PosArgs, []string{"foo", "bar", "--", "--"})

	args = au.ClassifyArgs([]string{"-p", "foo", "-f", "bar", "--foo"})
	assert.Equal(t, args.UnknownArgs, []string{"-p", "foo", "-f", "bar", "--foo"})
	assert.Equal(t, args.PosArgs, []string{})

	args = au.ClassifyArgs([]string{"xx", "-p", "foo", "-f", "bar", "--foo"})
	assert.Equal(t, args.UnknownArgs, []string{"-p", "foo", "-f", "bar", "--foo"})
	assert.Equal(t, args.PosArgs, []string{"xx"})

	args = au.ClassifyArgs([]string{"-p", "foo", "--", "-f", "bar", "--foo"})
	assert.Equal(t, args.UnknownArgs, []string{"-p", "foo"})
	assert.Equal(t, args.PosArgs, []string{"--", "-f", "bar", "--foo"})

	args = au.ClassifyArgs([]string{"xx", "-p", "foo", "--", "-f", "bar", "--foo"})
	assert.Equal(t, args.UnknownArgs, []string{"-p", "foo"})
	assert.Equal(t, args.PosArgs, []string{"xx", "--", "-f", "bar", "--foo"})
}

func TestCreateAliasArgs(t *testing.T) {
	// Simple case
	newArgs, err := au.CreateAliasArgs([]string{}, "tgt foo")
	assert.Nil(t, err)
	assert.Equal(t, newArgs, []string{"tgt", "foo"})
	// Check quotes
	newArgs, err = au.CreateAliasArgs([]string{"bla"}, "tgt 'foo'")
	assert.Nil(t, err)
	assert.Equal(t, newArgs, []string{"tgt", "foo", "bla"})
	// Sub in positional args at front
	newArgs, err = au.CreateAliasArgs([]string{"bla1", "bla2"}, "tgt '$2 foo'")
	assert.Nil(t, err)
	assert.Equal(t, newArgs, []string{"tgt", "bla2 foo", "bla1"})
	// Sub in positional args at end
	newArgs, err = au.CreateAliasArgs([]string{"bla1", "bla2"}, "tgt 'foo $2'")
	assert.Nil(t, err)
	assert.Equal(t, newArgs, []string{"tgt", "foo bla2", "bla1"})
	// Explicit remainder placement in middle
	newArgs, err = au.CreateAliasArgs([]string{"bla1", "bla2", "-n"}, "tgt $R '$2 foo'")
	assert.Nil(t, err)
	assert.Equal(t, newArgs, []string{"tgt", "-n", "bla1", "bla2 foo"})
	// Explicit remainder placement in middle 2
	newArgs, err = au.CreateAliasArgs([]string{"bla1", "bla2", "-n"}, "tgt $R foo")
	assert.Nil(t, err)
	assert.Equal(t, newArgs, []string{"tgt", "-n", "bla1", "bla2", "foo"})
	// Explicit remainder placement at end
	newArgs, err = au.CreateAliasArgs([]string{"bla1", "bla2"}, "tgt '$2 foo' $R")
	assert.Nil(t, err)
	assert.Equal(t, newArgs, []string{"tgt", "bla2 foo", "bla1"})
	// Positional arg ref which does not exist
	newArgs, err = au.CreateAliasArgs([]string{"bla1", "bla2"}, "tgt '$3 foo'")
	assert.Nil(t, err)
	assert.Equal(t, newArgs, []string{"tgt", " foo", "bla1", "bla2"})
}
