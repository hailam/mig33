package nativecodec

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/protocol"
)

func serviceTextField(p *protocol.Packet, tag int16) commands.Field[string] {
	value, valid := p.Text(tag)
	_, present := p.Fields[tag]
	return commands.Field[string]{Value: value, Present: present, Valid: valid}
}

func serviceByteField(p *protocol.Packet, tag int16) commands.Field[byte] {
	value, valid := p.Byte(tag)
	_, present := p.Fields[tag]
	return commands.Field[byte]{Value: value, Present: present, Valid: valid}
}

func DecodeServices(p *protocol.Packet) (*commands.Request, bool) {
	var input commands.Command
	switch p.Opcode {
	case 504:
		input = commands.UploadPhoto{Data: append([]byte(nil), p.Fields[2]...), Purpose: serviceByteField(p, 5)}
	case 602:
		input = commands.SetAvatar{Key: serviceTextField(p, 1)}
	case 800:
		input = commands.RecordCallback{
			Source:      serviceTextField(p, 1),
			Destination: serviceTextField(p, 2),
			Mode:        serviceByteField(p, 4),
			Confirm:     serviceByteField(p, 5),
		}
	case 900:
		input = commands.InviteMobile{Phone: serviceTextField(p, 1), Name: serviceTextField(p, 2)}
	case 206:
		input = commands.ConnectDirectory{ID: serviceByteField(p, 1), Mode: serviceByteField(p, 2), Option: serviceByteField(p, 3)}
	case 302:
		input = commands.DisconnectDirectory{ID: serviceByteField(p, 1)}
	case 907:
		input = commands.RemoveDirectorySettings{ID: serviceByteField(p, 30), Mode: serviceByteField(p, 1)}
	case 913:
		input = commands.MediaToken{Name: serviceTextField(p, 1)}
	case 921:
		input = commands.SubscribeActivity{Mode: serviceByteField(p, 1)}
	case 925:
		input = commands.ChooseLanguage{Code: serviceTextField(p, 1)}
	case 926:
		input = commands.ProviderIcons{IDs: append([]byte(nil), p.Fields[1]...)}
	default:
		return nil, false
	}

	return &commands.Request{Correlation: p.Transaction, Input: input}, true
}

func (c *Codec) EncodeServices(ctx context.Context, event *events.Event, publicURL string) (*protocol.Packet, bool) {
	switch payload := event.Data.(type) {
	case events.Resource:
		return protocol.New(914, event.Correlation).SetText(1, payload.Token).SetText(2, payload.Token).SetText(3, payload.Label).SetBytes(4, payload.Data).SetByte(5, payload.Type), true
	case events.EmoticonResource:
		item, ok := catalog.LookupEmoticon(payload.Token)
		if !ok {
			return protocol.New(0, event.Correlation).SetText(2, "Unknown downloadable emoticon"), true
		}

		data, err := c.emoticonContent(ctx, item)
		if err != nil {
			return protocol.New(0, event.Correlation).SetText(2, "Emoticon asset unavailable"), true
		}

		return protocol.New(914, event.Correlation).SetText(1, item.Token).SetText(2, item.Token).SetText(3, item.Label).SetBytes(4, data).SetByte(5, item.Type), true
	case events.Emoticons:
		return protocol.New(916, event.Correlation).SetText(1, strings.Join(payload.Tokens, " ")).SetText(2, ""), true
	case events.Theme:
		packet, err := c.storeThemePacket(ctx, payload.ID)
		if err != nil {
			return protocol.New(0, event.Correlation).SetText(2, "Theme unavailable"), true
		}

		packet.Transaction = event.Correlation
		return packet, true
	case events.Provider:
		return protocol.New(208, event.Correlation).SetByte(1, payload.ID).SetText(2, payload.Name+" saved contacts (local)").SetByte(3, payload.Channel).SetByte(4, 2).SetByte(5, 0), true
	case events.ProviderIcons:
		icon, err := c.readResource(ctx, "default-avatar.png")
		if err != nil {
			return protocol.New(0, event.Correlation).SetText(2, "Provider icons unavailable"), true
		}

		return protocol.New(927, event.Correlation).SetByte(1, payload.ID).SetBytes(2, icon).SetBytes(4, icon).SetBytes(5, icon).SetBytes(6, icon), true
	case events.ProviderRequestSaved:
		return protocol.New(5, event.Correlation).SetByte(1, 1).SetText(2, payload.Message).SetByte(3, 1).SetText(4, payload.Title), true
	case events.OpenFeature:
		return protocol.New(1, event.Correlation).SetText(1, featureURL(publicURL, payload.Controller, payload.Action)), true
	case events.Activity:
		age := (time.Now().UnixMilli() - payload.Timestamp) / 1000
		if age < 0 {
			age = 0
		}
		if age > 2147483 {
			age = 2147483
		}

		href := featureURL(publicURL, "lookout", "home") + "&username=" + url.QueryEscape(payload.Actor)
		body, err := pages.RenderFragment(pages.ActivitySummary(payload.Actor, payload.Display, href, payload.Text))
		if err != nil {
			return protocol.New(0, event.Correlation).SetText(2, "Activity unavailable"), true
		}

		packet := protocol.New(422, event.Correlation).SetInt(1, int32(age)).SetByte(2, 1).SetBytes(4, body)
		if payload.ImageID != 0 {
			packet.SetText(3, fmt.Sprintf("%s/assets/photos/%d", publicURL, payload.ImageID))
		}

		return packet, true
	default:
		return nil, false
	}
}

func featureURL(publicURL, controller, action string) string {
	return publicURL + "/sites/index.php?c=" + controller + "&a=" + action + "&v=midlet"
}

func (c *Codec) storeThemePacket(ctx context.Context, item string) (*protocol.Packet, error) {
	data, err := c.readResource(ctx, "default_v2")
	if err != nil {
		return nil, err
	}

	p, err := protocol.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if p.Opcode != 1000 {
		return nil, faults.ErrInvalid
	}

	p.Transaction = 0
	switch item {
	case "default":
		return p, nil
	case "theme-blue":
		p.SetText(0, "local_ocean_blue").SetText(2, "Ocean blue (local)")
		p.SetInt(139, 0xf0f7ff).SetInt(142, 0x005599).SetInt(143, 0x112244).SetInt(144, 0x005599).SetInt(145, 0x112244)
	case "theme-orange":
		p.SetText(0, "local_sunset_orange").SetText(2, "Sunset orange (local)")
		p.SetInt(139, 0xfff6ed).SetInt(142, 0x993300).SetInt(143, 0x332211).SetInt(144, 0x993300).SetInt(145, 0x332211)
	default:
		return nil, faults.ErrInvalid
	}

	return p, nil
}

const LanguagePrefix = "/sites/resources/language/v4.6/"

type Resource struct {
	ContentType string
	Data        []byte
}

func (c *Codec) LanguageResource(ctx context.Context, publicURL, name string) (Resource, error) {
	if err := ctx.Err(); err != nil {
		return Resource{}, err
	}

	var data []byte
	switch name {
	case "l":
		packet := protocol.New(1002, 0).SetText(1, "en-US").SetText(2, "English (US)").SetText(3, publicURL+LanguagePrefix+"en-US")
		var encoded bytes.Buffer
		if err := protocol.Encode(&encoded, packet); err != nil {
			return Resource{}, faults.WrapError(faults.Internal, "Language catalog unavailable", err)
		}

		data = encoded.Bytes()
	case "en-US":
		var err error
		data, err = c.readResource(ctx, "en-US")
		if err != nil {
			return Resource{}, err
		}
	default:
		return Resource{}, faults.NewError(faults.NotFound, "Resource not found")
	}

	return Resource{ContentType: "application/octet-stream", Data: data}, nil
}
