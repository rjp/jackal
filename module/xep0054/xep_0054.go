/*
 * Copyright (c) 2018 Miguel Ángel Ortuño.
 * See the LICENSE file for more information.
 */

package xep0054

import (
	"github.com/ortuman/jackal/log"
	"github.com/ortuman/jackal/storage"
	"github.com/ortuman/jackal/stream/c2s"
	"github.com/ortuman/jackal/xml"
)

const vCardNamespace = "vcard-temp"

// XEPVCard represents a vCard server stream module.
type XEPVCard struct {
	stm     c2s.Stream
	actorCh chan func()
}

// New returns a vCard IQ handler module.
func New(stm c2s.Stream) *XEPVCard {
	v := &XEPVCard{
		stm:     stm,
		actorCh: make(chan func(), 32),
	}
	if stm != nil {
		go v.actorLoop(stm.Context().Done())
	}
	return v
}

// AssociatedNamespaces returns namespaces associated
// with vCard module.
func (x *XEPVCard) AssociatedNamespaces() []string {
	return []string{vCardNamespace}
}

// MatchesIQ returns whether or not an IQ should be
// processed by the vCard module.
func (x *XEPVCard) MatchesIQ(iq *xml.IQ) bool {
	return (iq.IsGet() || iq.IsSet()) && iq.Elements().ChildNamespace("vCard", vCardNamespace) != nil
}

// ProcessIQ processes a vCard IQ taking according actions
// over the associated stream.
func (x *XEPVCard) ProcessIQ(iq *xml.IQ) {
	x.actorCh <- func() {
		vCard := iq.Elements().ChildNamespace("vCard", vCardNamespace)
		if iq.IsGet() {
			x.getVCard(vCard, iq)
		} else if iq.IsSet() {
			x.setVCard(vCard, iq)
		}
	}
}

func (x *XEPVCard) actorLoop(doneCh <-chan struct{}) {
	for {
		select {
		case f := <-x.actorCh:
			f()
		case <-doneCh:
			return
		}
	}
}

func (x *XEPVCard) getVCard(vCard xml.XElement, iq *xml.IQ) {
	if vCard.Elements().Count() > 0 {
		x.stm.SendElement(iq.BadRequestError())
		return
	}
	toJid := iq.ToJID()

	var username string
	if toJid.IsServer() {
		username = x.stm.Username()
	} else {
		username = toJid.Node()
	}

	resElem, err := storage.Instance().FetchVCard(username)
	if err != nil {
		log.Errorf("%v", err)
		x.stm.SendElement(iq.InternalServerError())
		return
	}
	log.Infof("retrieving vcard... (%s/%s)", x.stm.Username(), x.stm.Resource())

	resultIQ := iq.ResultIQ()
	if resElem != nil {
		resultIQ.AppendElement(resElem)
	} else {
		// empty vCard
		resultIQ.AppendElement(xml.NewElementNamespace("vCard", vCardNamespace))
	}
	x.stm.SendElement(resultIQ)
}

func (x *XEPVCard) setVCard(vCard xml.XElement, iq *xml.IQ) {
	toJid := iq.ToJID()
	if toJid.IsServer() || (toJid.IsBare() && toJid.Node() == x.stm.Username()) {
		log.Infof("saving vcard... (%s/%s)", x.stm.Username(), x.stm.Resource())

		err := storage.Instance().InsertOrUpdateVCard(vCard, x.stm.Username())
		if err != nil {
			log.Errorf("%v", err)
			x.stm.SendElement(iq.InternalServerError())
			return
		}
		x.stm.SendElement(iq.ResultIQ())
	} else {
		x.stm.SendElement(iq.ForbiddenError())
	}
}
