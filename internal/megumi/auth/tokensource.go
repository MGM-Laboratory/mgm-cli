package auth

import (
	"sync"

	"golang.org/x/oauth2"

	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/credstore"
	"github.com/MGM-Laboratory/mgm-cli/internal/megumi/oidc"
)

// persistingSource wraps an auto-refreshing oauth2.TokenSource and writes any
// rotated token back to the credential store, re-deriving display identity from
// the fresh access token.
type persistingSource struct {
	base  oauth2.TokenSource
	store *credstore.Store
	cfg   oidc.Config

	mu    sync.Mutex
	creds *credstore.Credentials
}

func (p *persistingSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if tok.AccessToken != p.creds.AccessToken {
		updated := tokenToCreds(tok, p.creds, p.cfg)
		if err := p.store.Save(updated); err != nil {
			return nil, err
		}
		p.creds = updated
	}
	return tok, nil
}

// current returns the most recently persisted credentials.
func (p *persistingSource) current() *credstore.Credentials {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.creds
}

// credsToToken builds an oauth2.Token from stored credentials.
func credsToToken(c *credstore.Credentials) *oauth2.Token {
	tok := &oauth2.Token{
		AccessToken:  c.AccessToken,
		RefreshToken: c.RefreshToken,
		TokenType:    c.TokenType,
		Expiry:       c.Expiry,
	}
	if c.IDToken != "" {
		tok = tok.WithExtra(map[string]any{"id_token": c.IDToken})
	}
	return tok
}

// tokenToCreds converts an oauth2.Token into a stored credential record,
// carrying forward id_token / identity from prev when the token doesn't supply
// them (e.g. a refresh response without a new id_token).
func tokenToCreds(tok *oauth2.Token, prev *credstore.Credentials, cfg oidc.Config) *credstore.Credentials {
	c := &credstore.Credentials{
		Method:       credstore.MethodAccount,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		Expiry:       tok.Expiry,
		Issuer:       cfg.Issuer,
	}
	if id, ok := tok.Extra("id_token").(string); ok && id != "" {
		c.IDToken = id
	} else if prev != nil {
		c.IDToken = prev.IDToken
	}
	if ident, err := ParseIdentity(tok.AccessToken, cfg); err == nil {
		c.Subject, c.Email, c.Name, c.Role = ident.Subject, ident.Email, ident.Name, ident.Role
	} else if prev != nil {
		c.Subject, c.Email, c.Name, c.Role = prev.Subject, prev.Email, prev.Name, prev.Role
	}
	return c
}
