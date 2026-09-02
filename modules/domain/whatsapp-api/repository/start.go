package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"go.mau.fi/whatsmeow"
)

type qrResp struct {
	qr  string
	err error
}

// A conta principal sobrevive a restarts num arquivo ao lado do store
// (mdtest.db). É só um número de telefone; sem conteúdo sensível.
const arquivoPrincipal = "conta_principal.txt"

func (r *repository) carregarPrincipal() {
	salvo := ""
	if b, err := os.ReadFile(arquivoPrincipal); err == nil {
		salvo = strings.TrimSpace(string(b))
	}

	if _, ok := r.sessoes[salvo]; ok {
		r.principal = salvo
		return
	}

	// Sem principal salva (ou ela foi desconectada): a primeira conta
	// existente assume, preservando o comportamento de conta única.
	nums := r.contasOrdenadas()
	if len(nums) > 0 {
		r.principal = nums[0]
		r.salvarPrincipal()
	} else {
		r.principal = ""
	}
}

func (r *repository) salvarPrincipal() {
	if err := os.WriteFile(arquivoPrincipal, []byte(r.principal), 0644); err != nil {
		log.Errorf("erro ao salvar a conta principal: %v", err)
	}
}

// DefinirPrincipal troca a conta usada pelas rotas legadas (a herança de
// todos os ambientes do Ravi).
func (r *repository) DefinirPrincipal(ctx context.Context, conta string) error {
	r.l.Lock()
	defer r.l.Unlock()
	if _, ok := r.sessoes[conta]; !ok {
		return fmt.Errorf("a conta %s não está conectada neste servidor", conta)
	}
	r.principal = conta
	r.salvarPrincipal()
	log.Infof("conta principal agora é %s", conta)
	return nil
}

// slotPareamento devolve (criando se preciso) o slot de pareamento de uma
// conta nova, com um device zerado do store.
func (r *repository) slotPareamento() *client {
	r.l.Lock()
	defer r.l.Unlock()
	if r.pareando == nil {
		r.pareando = &client{}
	}
	return r.pareando
}

// adotarPareado promove o slot de pareamento a sessão de verdade assim que
// o WhatsApp conclui o pareamento (Store.ID preenchido).
func (r *repository) adotarPareado(cli *whatsmeow.Client) {
	// O evento "success" chega logo antes de o whatsmeow terminar de gravar
	// o device; espera curta até o ID aparecer.
	for i := 0; i < 100 && cli.Store.ID == nil; i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if cli.Store.ID == nil {
		log.Errorf("pareamento sinalizou sucesso mas o device não apareceu no store")
		return
	}

	numero := cli.Store.ID.User

	r.l.Lock()
	defer r.l.Unlock()

	// Re-parear um número que já tem sessão: descarta a sessão antiga.
	if antiga, ok := r.sessoes[numero]; ok {
		if velho, err := antiga.Client(); err == nil && velho != cli {
			velho.Disconnect()
		}
	}

	sess := &client{}
	sess.SetClient(cli)
	r.sessoes[numero] = sess
	if r.pareando != nil {
		if pcli, err := r.pareando.Client(); err == nil && pcli == cli {
			r.pareando = nil
		}
	}

	// Primeira conta pareada vira a principal automaticamente (é o fluxo
	// clássico de conta única).
	if r.principal == "" {
		r.principal = numero
		r.salvarPrincipal()
	}

	log.Infof("conta %s pareada e ativa", numero)
}

func (r *repository) listenQrcode(cli *whatsmeow.Client, slot *client, chRet chan<- qrResp) {
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
					slot.SetQrCode(evt.Code)
					if first {
						chRet <- qrResp{qr: evt.Code}
						first = false
					}
				case "success":
					// Pareado: o whatsmeow grava o device no store; a sessão
					// é promovida a conta ativa.
					slot.SetQrCode("")
					go r.adotarPareado(cli)
				default:
					// timeout / err-*: a sessão de QR terminou. Zera o código
					// para que o próximo getQRCode inicie uma sessão nova.
					slot.SetQrCode("")
					if first {
						chRet <- qrResp{qr: "", err: fmt.Errorf("%s", evt.Event)}
						first = false
					}
				}
			}
			// Canal fechado pelo whatsmeow: garante o QR limpo.
			slot.SetQrCode("")
		}()
	}
}

// RequestNewQRCode inicia (ou continua) o pareamento de uma conta nova.
// No fluxo legado (conta única), se a principal já está logada devolve o
// aviso clássico em vez de parear outra conta sem o operador pedir.
func (r *repository) RequestNewQRCode(ctx context.Context, contaNova bool) (string, error) {
	if !contaNova {
		// Existir sessão principal = já há conta pareada (mesmo que
		// momentaneamente offline, o whatsmeow reconecta sozinho). O fluxo
		// legado nunca deve parear uma segunda conta por conta própria.
		if _, err := r.sessao(""); err == nil {
			return "já está logado", nil
		}
	}

	slot := r.slotPareamento()

	if qr := slot.getQrCode(); qr != "" {
		return qr, nil
	}

	if !atomic.CompareAndSwapUint32(&slot._qrCodeLock, 0, 1) {
		log.Errorf(" solicitação de QRCode já está em execução")
		return slot.getQrCode(), nil
	}

	defer atomic.StoreUint32(&slot._qrCodeLock, 0)

	// Reverifica sob o lock: outra requisição pode ter populado o QR.
	if qr := slot.getQrCode(); qr != "" {
		return qr, nil
	}

	// Encerra a sessão de pareamento anterior (não pareada) antes de abrir
	// uma nova. Disconnect (e não Logout) para não apagar credenciais nem
	// poluir o log com "store doesn't contain a device JID".
	if velho, err := slot.Client(); err == nil {
		velho.Disconnect()
	}

	cli := r.novoCliente(r._storage.NewDevice())
	slot.SetClient(cli)

	ch := make(chan qrResp, 1)
	r.listenQrcode(cli, slot, ch)

	if err := cli.Connect(); err != nil {
		go cli.Disconnect()
		return "", err
	}

	resp := <-ch
	if resp.err != nil {
		slot.SetQrCode("")
		return "", resp.err
	}

	// O QR em cache é mantido atualizado pela goroutine de listenQrcode
	// enquanto a sessão durar (sem expiração destrutiva a cada 20s).
	slot.SetQrCode(resp.qr)

	return resp.qr, nil
}
