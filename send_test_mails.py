#!/usr/bin/env python3
"""
MailCraft Test-Sender
=====================

Verschickt eine ganze Palette unterschiedlicher E-Mails an den MailCraft
SMTP-Server, damit man im Web-UI alle Darstellungs- und Parsing-Faelle
vergleichen kann (Plain vs. HTML, Attachments, Unicode, Sanitizing, ...).

Benutzung:
    python3 send_test_mails.py                # alle Szenarien senden
    python3 send_test_mails.py --list         # verfuegbare Szenarien anzeigen
    python3 send_test_mails.py html alt       # nur bestimmte Szenarien senden
    python3 send_test_mails.py --host 127.0.0.1 --port 1025
    python3 send_test_mails.py --auth user pass        # mit AUTH LOGIN/PLAIN
    python3 send_test_mails.py --starttls              # STARTTLS verwenden

Hinweis: MailCraft bereinigt eingehendes HTML serverseitig (bluemonday
UGCPolicy). Das Szenario "unsafe" zeigt, was dabei gefiltert wird.
"""

import argparse
import base64
import itertools
import smtplib
import sys
from email.message import EmailMessage
from email.utils import formatdate, make_msgid

# 1x1 PNG (roter Pixel) - reicht, um Inline-/Anhang-Bilder zu testen.
_PNG_1PX = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mP8z"
    "8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
)

# Minimales gueltiges PDF, damit ein "echter" PDF-Anhang ankommt.
_PDF_BYTES = (
    b"%PDF-1.1\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n"
    b"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n"
    b"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 200]>>endobj\n"
    b"trailer<</Root 1 0 R>>\n%%EOF\n"
)

DEFAULT_TO = "empfaenger@example.com"

# Pool unterschiedlicher Absender - wird der Reihe nach durchrotiert,
# damit jede Test-Mail einen anderen From-Namen bekommt.
SENDERS = [
    "Anna Schmidt <anna.schmidt@example.com>",
    "Newsletter Team <news@shop.example>",
    "Max Mustermann <max.mustermann@firma.de>",
    "Support <support@helpdesk.example>",
    "Julia Wagner <j.wagner@beispiel.org>",
    "GitHub <notifications@github.example>",
    "Rechnung <billing@payments.example>",
    "Dr. Lukas Becker <becker@klinik.example>",
    "Marketing <hello@brand.example>",
    "李雷 <li.lei@example.cn>",
    "no-reply <no-reply@service.example>",
    "Tom & Jerry GmbH <kontakt@tomjerry.example>",
    "MailCraft Tester <tester@example.com>",
]
_sender_cycle = itertools.cycle(SENDERS)


def _base(subject, sender=None):
    """Erzeugt eine EmailMessage mit den gemeinsamen Standard-Headern."""
    msg = EmailMessage()
    msg["Subject"] = subject
    msg["From"] = sender or next(_sender_cycle)
    msg["To"] = DEFAULT_TO
    msg["Date"] = formatdate(localtime=True)
    msg["Message-ID"] = make_msgid(domain="example.com")
    return msg


# --------------------------------------------------------------------------
# Szenarien
# --------------------------------------------------------------------------

def build_plain():
    """Reine Text-Mail (text/plain)."""
    msg = _base("01 - Plain Text")
    msg.set_content(
        "Hallo!\n\n"
        "Dies ist eine ganz normale Nur-Text E-Mail.\n"
        "Zeilenumbrueche und Absaetze sollten erhalten bleiben.\n\n"
        "Gruss,\nMailCraft Tester"
    )
    return msg


def build_html():
    """Reine HTML-Mail (text/html, ohne Text-Alternative)."""
    msg = _base("02 - Nur HTML")
    msg.set_content("Fallback-Text falls kein HTML angezeigt wird.")
    msg.add_alternative(
        """\
<html><body style="font-family: Arial, sans-serif; color:#222;">
  <h1 style="color:#3b82f6;">Nur-HTML Mail</h1>
  <p>Diese Mail hat <b>fett</b>, <i>kursiv</i> und <u>unterstrichen</u>.</p>
  <ul><li>Punkt eins</li><li>Punkt zwei</li><li>Punkt drei</li></ul>
  <p><a href="https://example.com">Ein Link</a></p>
</body></html>""",
        subtype="html",
    )
    return msg


def build_alternative():
    """multipart/alternative: Text + reiches HTML zum Vergleichen."""
    msg = _base("03 - Text + HTML (alternative)")
    msg.set_content(
        "PLAIN-VERSION\n\n"
        "Das ist die Nur-Text Variante derselben Nachricht.\n"
        "Im HTML-Tab sollte stattdessen eine formatierte Tabelle erscheinen."
    )
    msg.add_alternative(
        """\
<html><body style="font-family: -apple-system, Arial, sans-serif;">
  <h2 style="margin:0 0 12px;">HTML-Version</h2>
  <p>Vergleiche diesen Tab mit der Plain-Text Ansicht.</p>
  <table border="1" cellpadding="8" cellspacing="0"
         style="border-collapse:collapse;">
    <tr style="background:#f3f4f6;"><th>Produkt</th><th>Preis</th></tr>
    <tr><td>Kaffee</td><td>2,50 &euro;</td></tr>
    <tr><td>Tee</td><td>1,80 &euro;</td></tr>
  </table>
  <blockquote style="border-left:4px solid #3b82f6;padding-left:12px;color:#555;">
    Ein Zitat zum Testen des Renderings.
  </blockquote>
</body></html>""",
        subtype="html",
    )
    return msg


def build_unsafe():
    """HTML mit gefaehrlichem Inhalt - zeigt, was MailCraft wegsanitized."""
    msg = _base("04 - Unsafe HTML (Sanitizer-Test)")
    msg.set_content("Plain-Fallback fuer die Sanitizer-Testmail.")
    msg.add_alternative(
        """\
<html><body>
  <h1>Sanitizer Test</h1>
  <p>Sichtbarer normaler Absatz.</p>
  <script>alert('XSS - sollte entfernt werden');</script>
  <style>body { background: red; }</style>
  <p style="color:green;" onclick="alert('inline')">
    Absatz mit inline-style und onclick (wird vermutlich entschaerft).
  </p>
  <a href="javascript:alert('boese')">javascript-Link</a><br>
  <img src="x" onerror="alert('img')" alt="kaputtes Bild">
  <iframe src="https://example.com"></iframe>
  <p>Letzter sichtbarer Absatz.</p>
</body></html>""",
        subtype="html",
    )
    return msg


def build_inline_image():
    """multipart/related: HTML mit eingebettetem Inline-Bild (cid)."""
    msg = _base("05 - Inline-Bild (cid)")
    msg.set_content("Diese Mail enthaelt ein eingebettetes Inline-Bild.")
    cid = make_msgid(domain="example.com")[1:-1]  # ohne < >
    msg.add_alternative(
        f"""\
<html><body>
  <h2>Inline-Bild Test</h2>
  <p>Darunter sollte ein (winziges, skaliertes) Bild erscheinen:</p>
  <img src="cid:{cid}" width="80" height="80"
       style="border:2px solid #3b82f6;" alt="inline">
</body></html>""",
        subtype="html",
    )
    # Bild an den HTML-Teil haengen, damit cid:-Referenz funktioniert.
    html_part = msg.get_payload()[1]
    html_part.add_related(_PNG_1PX, maintype="image", subtype="png", cid=f"<{cid}>")
    return msg


def build_attachment():
    """Mail mit einem einzelnen PDF-Anhang."""
    msg = _base("06 - Ein Anhang (PDF)")
    msg.set_content("Im Anhang findest du ein kleines Test-PDF.")
    msg.add_attachment(
        _PDF_BYTES, maintype="application", subtype="pdf", filename="bericht.pdf"
    )
    return msg


def build_multi_attachments():
    """Mail mit mehreren Anhaengen unterschiedlichen Typs."""
    msg = _base("07 - Mehrere Anhaenge")
    msg.set_content("Diese Mail enthaelt mehrere Anhaenge: txt, csv, png, pdf.")
    msg.add_attachment(
        "Spalte A;Spalte B\n1;2\n3;4\n".encode("utf-8"),
        maintype="text", subtype="csv", filename="daten.csv",
    )
    msg.add_attachment(
        "Eine einfache Textdatei.\nMit zwei Zeilen.\n".encode("utf-8"),
        maintype="text", subtype="plain", filename="notiz.txt",
    )
    msg.add_attachment(
        _PNG_1PX, maintype="image", subtype="png", filename="pixel.png"
    )
    msg.add_attachment(
        _PDF_BYTES, maintype="application", subtype="pdf", filename="dokument.pdf"
    )
    return msg


def build_cc_bcc():
    """Mail mit mehreren To-, CC- und BCC-Empfaengern."""
    msg = _base("08 - CC und BCC")
    del msg["To"]
    msg["To"] = "erster@example.com, zweiter@example.com"
    msg["Cc"] = "kopie1@example.com, kopie2@example.com"
    msg["Bcc"] = "blindkopie@example.com"
    msg["Reply-To"] = "antwort@example.com"
    msg.set_content(
        "Diese Mail geht an mehrere To-, CC- und BCC-Empfaenger.\n"
        "Pruefe im UI, ob CC korrekt angezeigt wird."
    )
    return msg


def build_headers():
    """Mail mit vielen benutzerdefinierten Headern."""
    msg = _base("09 - Custom Headers")
    msg["X-Priority"] = "1 (Highest)"
    msg["Importance"] = "high"
    msg["X-Mailer"] = "MailCraft-Tester/1.0"
    msg["X-Custom-Tag"] = "integration-test"
    msg["List-Unsubscribe"] = "<https://example.com/unsubscribe>"
    msg["X-Spam-Score"] = "0.1"
    msg.set_content(
        "Diese Mail traegt diverse Custom-Header.\n"
        "Im UI im Header-/Raw-Tab pruefbar."
    )
    return msg


def build_unicode():
    """Unicode-Test: Umlaute, Emojis, nicht-lateinische Schrift."""
    msg = _base("10 - Unicode äöü 日本語 😀🚀")
    del msg["To"]
    msg["To"] = "Müller <mueller@example.com>"
    msg.set_content(
        "Umlaute: äöü ÄÖÜ ß\n"
        "Emojis: 😀 🚀 ✉️ ✅\n"
        "Japanisch: こんにちは世界\n"
        "Griechisch: Καλημέρα\n"
        "Kyrillisch: Привет мир\n"
    )
    return msg


def build_long():
    """Lange Mail (viele Zeilen), um Scrollen/Performance zu testen."""
    msg = _base("11 - Lange Nachricht")
    lines = [f"Zeile {i:04d}: Lorem ipsum dolor sit amet, consetetur." for i in range(1, 401)]
    msg.set_content("\n".join(lines))
    return msg


def build_empty_body():
    """Mail mit leerem Body, nur Subject."""
    msg = _base("12 - Leerer Body")
    msg.set_content("")
    return msg


def build_full():
    """Alles kombiniert: Text + HTML + Inline-Bild + Anhang + CC + Header."""
    msg = _base("13 - Komplettpaket")
    msg["Cc"] = "kopie@example.com"
    msg["X-Custom-Tag"] = "komplett"
    msg.set_content(
        "Plain-Text Teil des Komplettpakets.\n"
        "Enthaelt HTML-Alternative, Inline-Bild und einen Anhang."
    )
    cid = make_msgid(domain="example.com")[1:-1]
    msg.add_alternative(
        f"""\
<html><body style="font-family:Arial,sans-serif;">
  <h1 style="color:#10b981;">Komplettpaket</h1>
  <p>Text, HTML, Inline-Bild und Anhang in einer Mail.</p>
  <img src="cid:{cid}" width="60" height="60" alt="inline"><br>
  <p>Anbei zusaetzlich ein PDF.</p>
</body></html>""",
        subtype="html",
    )
    html_part = msg.get_payload()[1]
    html_part.add_related(_PNG_1PX, maintype="image", subtype="png", cid=f"<{cid}>")
    msg.add_attachment(
        _PDF_BYTES, maintype="application", subtype="pdf", filename="anhang.pdf"
    )
    return msg


# Registry: Name -> Builder-Funktion
SCENARIOS = {
    "plain": build_plain,
    "html": build_html,
    "alt": build_alternative,
    "unsafe": build_unsafe,
    "inline": build_inline_image,
    "attachment": build_attachment,
    "multi": build_multi_attachments,
    "ccbcc": build_cc_bcc,
    "headers": build_headers,
    "unicode": build_unicode,
    "long": build_long,
    "empty": build_empty_body,
    "full": build_full,
}


def main():
    parser = argparse.ArgumentParser(
        description="Verschickt Test-E-Mails an MailCraft.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("scenarios", nargs="*",
                        help="Szenarien (Standard: alle). Siehe --list.")
    parser.add_argument("--host", default="localhost", help="SMTP-Host (Standard: localhost)")
    parser.add_argument("--port", type=int, default=1025, help="SMTP-Port (Standard: 1025)")
    parser.add_argument("--list", action="store_true", help="Szenarien auflisten und beenden")
    parser.add_argument("--auth", nargs=2, metavar=("USER", "PASS"),
                        help="Mit SMTP-AUTH einloggen")
    parser.add_argument("--starttls", action="store_true", help="STARTTLS verwenden")
    args = parser.parse_args()

    if args.list:
        print("Verfuegbare Szenarien:")
        for name, fn in SCENARIOS.items():
            print(f"  {name:12s} - {(fn.__doc__ or '').strip().splitlines()[0]}")
        return 0

    selected = args.scenarios or list(SCENARIOS.keys())
    unknown = [s for s in selected if s not in SCENARIOS]
    if unknown:
        print(f"Unbekannte Szenarien: {', '.join(unknown)}", file=sys.stderr)
        print("Nutze --list fuer die Liste.", file=sys.stderr)
        return 2

    try:
        with smtplib.SMTP(args.host, args.port, timeout=10) as smtp:
            if args.starttls:
                smtp.starttls()
            if args.auth:
                smtp.login(args.auth[0], args.auth[1])

            for name in selected:
                msg = SCENARIOS[name]()
                smtp.send_message(msg)
                print(f"  + gesendet: {name:12s} - {msg['Subject']}")
    except (smtplib.SMTPException, OSError) as exc:
        print(f"Fehler beim Senden: {exc}", file=sys.stderr)
        return 1

    print(f"\nFertig. {len(selected)} Mail(s) an {args.host}:{args.port} gesendet.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
