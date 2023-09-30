package repository

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type qrResp struct {
	qr  string
	err error
}

func (r *repository) getDevice() (*store.Device, error) {
	return r._storage.GetFirstDevice()

}

func (r *repository) _connToWhatsApp() (*whatsmeow.Client, error) {
	log.Infof("Iniciando conexão")

	r.conn.SetQrCode("")

	wac, err := r.newConn()
	if err != nil {
		log.Errorf("erro ao conectar ao WhatsApp Socket, verifique a internet, err: %s", err)
		return nil, err
	}

	log.Infof("API conectou ao WhatsApp")
	return wac, nil
}

func (r *repository) newConn() (*whatsmeow.Client, error) {
	device, err := r.getDevice()
	if err != nil {
		return nil, err
	}

	cli := whatsmeow.NewClient(device, waLog.Stdout("Client", r._logLevel, true))

	cli.AutomaticMessageRerequestFromPhone = true
	cli.AddEventHandler(r.handler)
	return cli, nil
}

func (r *repository) listenQrcode(cli *whatsmeow.Client, chRet chan<- qrResp) {
	ch, err := cli.GetQRChannel(context.Background())
	if err != nil {
		if errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
			chRet <- qrResp{
				qr:  "",
				err: fmt.Errorf("já está logado"),
			}
		} else {
			chRet <- qrResp{
				qr:  "",
				err: err,
			}
		}
	} else {
		go func() {
			for evt := range ch {
				if evt.Event == "code" {
					chRet <- qrResp{
						qr: evt.Code,
					}
					return
				} else {
					chRet <- qrResp{
						qr:  "",
						err: fmt.Errorf("%s", evt.Event),
					}
				}
			}
		}()
	}
}

func (r *repository) RequestNewQRCode(ctx context.Context) (string, error) {
	if cli, err := r.conn.Client(); err == nil {
		if cli.IsLoggedIn() {
			return "já está logado", nil
		}
	}

	if qr := r.conn.getQrCode(); qr != "" {
		return qr, nil
	}

	if !atomic.CompareAndSwapUint32(&r.conn._qrCodeLock, 0, 1) {
		log.Errorf(" solicitação de QRCode já está em execução")
		return r.conn.getQrCode(), nil
	}

	defer atomic.StoreUint32(&r.conn._qrCodeLock, 0)

	conn, err := r.conn.Client()
	if err == nil {
		err = conn.Logout()
		if err != nil {
			log.Errorf("erro ao desconectar, %s", err.Error())
		}
	}

	cli, err := r.newConn()
	if err != nil {
		log.Errorf("%s", err.Error())
		return "", err
	}

	r.conn.SetClient(cli)
	ch := make(chan qrResp, 1)
	r.listenQrcode(cli, ch)

	err = cli.Connect()
	if err != nil {
		go cli.Logout()
		return "", err
	}

	resp := <-ch
	if resp.err != nil {
		r.conn.SetQrCode("")
		return "", resp.err
	}

	go r.clearQr()
	r.conn.SetQrCode(resp.qr)

	return resp.qr, resp.err
}

func (r *repository) clearQr() {
	<-time.After(20 * time.Second)
	r.conn.SetQrCode("")
}
