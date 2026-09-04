#!/usr/bin/env python3
"""Traduz português para inglês offline, com o modelo Argos pt→en em CTranslate2.

Segundo passo do pipeline do Vox. O whisper transcreve português literal (tarefa
em que ele é confiável) e este script traduz, em vez de pedir ao whisper que
ouça e traduza de uma vez só — passo único que parafraseia o sentido e troca
palavras em fala rápida.

Se a tradução falhar por qualquer motivo, o texto original é devolvido intacto:
entregar o ditado no idioma errado é muito melhor que perdê-lo.

Precisa do venv em .local/venv-translate, que tem ctranslate2 e sentencepiece.
"""

import os
import re
import sys

project_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
default_model_dir = os.path.join(project_root, ".local", "models", "translate-pt-en")


def translate(text, model_dir=default_model_dir):
    import ctranslate2
    import sentencepiece

    tokenizer = sentencepiece.SentencePieceProcessor(
        os.path.join(model_dir, "sentencepiece.model")
    )
    translator = ctranslate2.Translator(
        os.path.join(model_dir, "model"),
        device="cpu",
        intra_threads=os.cpu_count() or 4,
    )
    # O modelo foi treinado frase a frase; alimentá-lo com um parágrafo inteiro
    # degrada bastante a saída.
    sentences = [s for s in re.split(r"(?<=[.!?])\s+", text) if s]
    results = translator.translate_batch(
        [tokenizer.encode(s, out_type=str) for s in sentences], beam_size=4
    )
    # O sentencepiece marca início de palavra com U+2581; juntar os pedaços e
    # trocar esse marcador por espaço reconstrói o texto.
    return " ".join(
        "".join(r.hypotheses[0]).replace("▁", " ").strip() for r in results
    )


def translate_or_passthrough(text, model_dir=default_model_dir):
    try:
        translated = translate(text, model_dir)
    except Exception as error:
        print(f"translate-pt-en: {error}", file=sys.stderr)
        return text
    return translated if translated.strip() else text


def self_check():
    original = "teste"
    assert translate_or_passthrough(original, "/nao/existe") == original
    print("ok: fallback preserva o texto de entrada")


if __name__ == "__main__":
    if "--self-check" in sys.argv:
        self_check()
        sys.exit(0)
    source = sys.stdin.read().strip()
    if source:
        # Sem quebra de linha no fim: o texto vai direto para a área de
        # transferência e um "\n" extra viraria Enter na janela de destino.
        sys.stdout.write(translate_or_passthrough(source))
