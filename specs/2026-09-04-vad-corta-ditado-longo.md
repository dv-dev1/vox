# Ditado longo chegava picotado

> O nome do arquivo é de quando a suspeita ainda era o VAD. A causa era outra.

## Intenção

Ditado longo chegava picotado: frases inteiras sumiam no meio, sem erro e sem
aviso. Uma fala de mais de um minuto voltava como uma linha só.

A causa é o `-nt` (`--no-timestamps`), que o Vox passava sempre. O token de
timestamp é a âncora que faz o decoder avançar a janela de 30 s; sem ele o
modelo perde a posição e pula trecho. Nada a ver com o modelo, com o prompt de
vocabulário ou com o idioma.

Medido em 116 s de fala contínua, 20 frases numeradas e distintas, mesmo áudio,
só o flag muda:

| configuração | frases recuperadas |
| --- | --- |
| `-nt` (o padrão antigo) | 4 de 20 |
| sem `-nt` | 20 de 20 |

O texto colado não muda de forma: os timestamps vivem em campos próprios do
JSON, fora de `transcription[].text`, que é o que o Vox lê.

Custo: a transcrição longa passou a levar ~21 s em vez de ~11 s. Não é
regressão — antes ela era rápida porque desistia de dois terços da fala.

O Silero VAD, que primeiro levou a culpa, é inocente: sob `-nt` ele piorava o
quadro (1 frase de 20), mas com o `-nt` fora entrega as mesmas 20 de 20. Ficou
desligado por padrão porque não paga — em ditado curto não economiza nada
(2536 ms contra 2569 ms) e no longo ficou ~1 s mais lento. `VOX_WHISPER_VAD=1`
religa.

> Ressalva: o áudio do teste é sintético (`espeak-ng`), então a qualidade das
> palavras não representa voz real. O que a tabela mede é quanto texto
> sobrevive, com o áudio idêntico nas duas linhas.

## Critério de aceite

As 20 frases numeradas sobrevivem à fala de 116 s:

```bash
cd ~/vox && VOX_WHISPER_TRANSLATE=0 ./vox transcribe /tmp/varied.wav 2>&1 \
  | grep -coiE 'fra[slz][ie]e? ?[0-9]+'
```

Esperado: `20`.

O texto colado não pode conter timestamp:

```bash
cd ~/vox && VOX_WHISPER_TRANSLATE=0 ./vox transcribe /tmp/varied.wav 2>&1 \
  | grep -c '\[00:'
```

Esperado: `0`.

Reconstruir o áudio do teste, se ele não existir mais:

```bash
espeak-ng -v pt-br -s 145 -w /tmp/v.wav -f specs/probe-ditado-longo.txt
ffmpeg -y -loglevel error -i /tmp/v.wav -ar 16000 -ac 1 -c:a pcm_s16le /tmp/varied.wav
```

## Fora de escopo

- Calibrar os parâmetros do Silero (`--vad-threshold`, `--vad-speech-pad-ms`).
  O VAD não é mais o gargalo e está desligado.
- Recuperar o tempo perdido. A transcrição longa dobrou de duração porque
  passou a transcrever a fala inteira; cortar isso é trocar texto por relógio de
  novo.
- Segmentar o ditado em pedaços para paralelizar. Só faz sentido se o tempo
  virar problema no uso real.
