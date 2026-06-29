package repository

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type qrResp struct {
	qr  string
	err error
}

func (r *repository) getDevice() (*store.Device, error) {
	return r._storage.GetFirstDevice(context.Background())

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
			first := true
			for evt := range ch {
				switch evt.Event {
				case "code":
					// Mantém o QR exibido sempre no código válido atual e,
					// principalmente, CONTINUA drenando o canal. Se pararmos
					// de ler, o whatsmeow desconecta o cliente no próximo
					// refresh (~20s) e o pareamento nunca conclui.
					r.conn.SetQrCode(evt.Code)
					if first {
						chRet <- qrResp{qr: evt.Code}
						first = false
					}
				case "success":
					// Pareado: o whatsmeow já gravou o device no store
					// (Store.ID != nil) e o status passa a "OK" sozinho.
					r.conn.SetQrCode("")
				default:
					// timeout / err-*: a sessão de QR terminou. Zera o código
					// para que o próximo getQRCode inicie uma sessão nova.
					r.conn.SetQrCode("")
					if first {
						chRet <- qrResp{qr: "", err: fmt.Errorf("%s", evt.Event)}
						first = false
					}
				}
			}
			// Canal fechado pelo whatsmeow: garante o QR limpo.
			r.conn.SetQrCode("")
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

	// Reverifica sob o lock: outra requisição pode ter populado o QR.
	if qr := r.conn.getQrCode(); qr != "" {
		return qr, nil
	}

	// Encerra a sessão anterior (não pareada) antes de abrir uma nova.
	// Disconnect (e não Logout) para não apagar credenciais nem poluir o log
	// com "store doesn't contain a device JID".
	if conn, err := r.conn.Client(); err == nil {
		conn.Disconnect()
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
		go cli.Disconnect()
		return "", err
	}

	resp := <-ch
	if resp.err != nil {
		r.conn.SetQrCode("")
		return "", resp.err
	}

	// O QR em cache é mantido atualizado pela goroutine de listenQrcode
	// enquanto a sessão durar (sem expiração destrutiva a cada 20s).
	r.conn.SetQrCode(resp.qr)

	return resp.qr, nil
}
