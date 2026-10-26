# Guía de integración con proveedor tecnológico DIAN

**Alcance:** contrato interno y diligencia técnica para integrar un proveedor tecnológico autorizado en Colombia.

**Actualizado:** 2026-10-23

**Estado:** diseño preliminar; falta recibir y validar la documentación privada de DATAICO y The Factory HKA.

**Límite:** esta guía no documenta endpoints de ningún proveedor ni acredita cumplimiento fiscal.

## Decisión pendiente

Para el MVP se eligió integrar un proveedor tecnológico autorizado, pero todavía no se seleccionó uno. El listado DIAN publicado como `Proveedores-habilitados-15102026.xlsx` incluye a DATAICO S.A.S. (NIT 901223648) y The Factory HKA Colombia S.A.S. (NIT 900390126).

La información pública encontrada confirma que:

| Candidato | Información pública útil | Información pendiente |
|---|---|---|
| DATAICO | Ofrece una API para software de facturación, nómina o POS que requiere transmitir documentos electrónicos a la DIAN. | Especificación, sandbox, autenticación, soporte multiempresa, callbacks, precios, límites y SLA. Su página indica que el precio de API no está incluido en los planes publicados y requiere cita comercial. |
| The Factory HKA | Describe un proceso de integración directa para software, publica un portal demo y enumera factura, notas, contingencia y otros documentos. | Paquete técnico, acceso al sandbox para integración, autenticación, modelo multiempresa, callbacks, idempotencia, precios y SLA. |

Ambos candidatos deben confirmar por escrito su autorización DIAN vigente para la entidad legal contratante y el alcance de documentos aplicable. No se enviaron solicitudes comerciales ni se aceptaron términos.

**Alegra no es finalista por ahora:** su API pública documenta operaciones de facturas y emisión, pero sus términos exigen autorización previa para distribuir o comercializar una implementación y restringen productos que compitan con Alegra. No integrarla sin autorización escrita que cubra este producto SaaS. Carvajal y Facture también aparecen en el listado DIAN, pero aún no se verificó una oferta de integración API que permita compararlos técnicamente.

## Estado actual del repositorio

- `IssueInvoiceHandler` cambia la factura de `draft` a `issued`; ese estado es comercial y **no significa** que el documento haya sido transmitido o validado por la DIAN.
- El evento `invoice.issued` se persiste en el outbox. `relay-worker` registra el evento; `pdf-service` genera la representación PDF desde el snapshot. Ninguno transmite a un proveedor.
- `InvoiceSnapshot` conserva datos comerciales y de partes, pero aún no tipa todos los datos fiscales colombianos requeridos por la operación.
- El PDF localizado es una representación interna, no el XML UBL ni una representación de un documento validado por la DIAN.
- El estado de factura hoy usa `draft`, `issued`, `paid`, `overdue` y `cancelled`; no representa el resultado fiscal.

La implementación debe preservar las reglas de `AGENTS.md` y `docs/agents/SAFETY.md`: aislamiento por tenant y RLS, importes en unidades menores enteras, tasas en puntos base, secretos fuera del código/logs, y no tratar una respuesta fallida o incompleta como éxito.

## Flujo objetivo

Los nombres siguientes describen el comportamiento esperado, no rutas o capacidades ya implementadas.

1. **Preparación:** validar los datos fiscales del emisor, adquirente, numeración y configuración del tenant contra las reglas acordadas con el proveedor. No permitir emitir si faltan datos obligatorios.
2. **Congelación:** crear un snapshot fiscal inmutable que incluya la información efectivamente enviada. Una corrección posterior no debe alterar el documento transmitido.
3. **Solicitud durable:** en una transacción, registrar una solicitud fiscal y su mensaje outbox. La API debe devolver que el proceso quedó pendiente, no afirmar aceptación DIAN.
4. **Envío asíncrono:** un consumidor aislado usa un adaptador del proveedor y una clave idempotente estable para enviar o reconciliar el documento.
5. **Conciliación:** persistir el identificador del proveedor, estado, marcas de tiempo y respuesta. Si el resultado es ambiguo por timeout, consultar estado antes de reenviar.
6. **Resultado:** ante aceptación, conservar los artefactos fiscales y habilitar su descarga; ante rechazo, mostrar errores accionables y mantener evidencia; ante indisponibilidad, modelar la contingencia conforme a la regulación y al procedimiento del proveedor.
7. **Correcciones:** notas crédito/débito deben ser documentos relacionados con la factura original. La cancelación comercial interna no reemplaza una nota ni revierte por sí sola una factura validada.

El ciclo fiscal debe mantenerse separado del ciclo comercial y de pagos. Estados fiscales candidatos para diseñar, sujetos al contrato y a revisión contable/jurídica:

`not_requested` → `queued` → `submitted` → `accepted` | `rejected` | `outcome_unknown` | `contingency`

No definir todavía una máquina de estados final ni mapear respuestas de proveedor hasta obtener su catálogo oficial de estados y reglas de transición.

## Límite de integración

Mantener la dependencia del proveedor detrás de un puerto de aplicación neutral. La siguiente forma es ilustrativa y no es un contrato ya presente en el código:

```go
type EInvoiceProvider interface {
    Submit(ctx context.Context, request SubmitRequest) (Submission, error)
    GetStatus(ctx context.Context, document ProviderDocumentID) (ProviderStatus, error)
    GetArtifacts(ctx context.Context, document ProviderDocumentID) (FiscalArtifacts, error)
}
```

El contrato definitivo deberá reflejar capacidades verificadas: webhooks o consulta, emisión individual o por lotes, notas, contingencia, obtención de XML/respuesta DIAN y límites de reintento. No construir un adaptador con endpoints especulativos.

La clave de idempotencia debe ser estable para el mismo documento fiscal y la misma intención de envío. Un reintento técnico nunca debe consumir un nuevo consecutivo ni crear una segunda factura. Si el proveedor no admite idempotencia, documentar cómo se reconcilia un timeout antes de habilitar reintentos automáticos.

## Seguridad y aislamiento multi-tenant

- Confirmar si el proveedor ofrece una cuenta de plataforma con asociación segura de emisores o requiere credenciales/cuenta por emisor. Esta decisión determina el modelo de secretos y onboarding.
- Guardar credenciales en el sistema de secretos del entorno, con rotación y acceso mínimo. No guardarlas en `tenant.settings`, el snapshot, respuestas API, eventos outbox ni logs.
- Asociar cada cuenta/identificador externo del proveedor a un tenant mediante datos internos protegidos por RLS. No derivar el tenant de un campo no verificado del webhook.
- Verificar firma/autenticidad, timestamp y protección contra replay de cada webhook; procesar eventos duplicados e inesperados sin duplicar transiciones.
- Registrar auditoría sin incluir secretos, XML completo ni datos personales innecesarios. Definir cifrado, retención, exportación y borrado con el proveedor y con asesoría legal.
- Aislar credenciales y configuración por ambiente; las credenciales de sandbox no pueden operar en producción.

## Paquete documental que se debe solicitar a DATAICO y HKA

No iniciar la integración de producción hasta tener y revisar:

1. **Identidad y habilitación:** razón social/NIT contratante, evidencia de habilitación DIAN vigente, documentos cubiertos y versión del anexo DIAN soportada.
2. **Especificación:** OpenAPI/WSDL vigente, versión/fecha, esquemas y ejemplos válidos/erróneos para factura, nota crédito y nota débito.
3. **Entornos:** URLs y credenciales de sandbox, procedimiento de habilitación, datos de prueba, set de certificación y criterios de aprobación.
4. **Seguridad:** autenticación, rotación/revocación de secretos, firma de webhooks, certificados digitales, cifrado y controles de acceso.
5. **Multiempresa:** onboarding de múltiples emisores, administración de resolución/rangos, mapeo de tenant y propiedad/portabilidad de datos y documentos.
6. **Ciclo y errores:** catálogo de estados, consulta/reconciliación, idempotencia, duplicados, rechazos, contingencias, cancelaciones, límites, timeouts y reintentos.
7. **Artefactos y conservación:** XML firmado, respuesta/validación DIAN, CUFE/QR, representación gráfica, API de descarga, retención, exportación y evidencia de entrega.
8. **Operación y contrato:** SLA y soporte, ventanas de mantenimiento, continuidad, recuperación, residencia/subprocesadores de datos, tratamiento de información, precios por documento/emisor, mínimos, límites y condiciones de salida.

Guardar cada respuesta y versión documental en el expediente del proveedor, con fecha, fuente y responsable de validación. Los puntos no respondidos permanecen como bloqueos; no se completan por inferencia a partir de material comercial.

### Plantilla de solicitud técnica

**Asunto:** Solicitud de documentación para integración SaaS con proveedor tecnológico DIAN

> Estamos evaluando integrar su servicio de facturación electrónica con una plataforma SaaS multiempresa. ¿Podrían compartir el paquete técnico vigente para integración directa, incluyendo especificación API, sandbox y casos de prueba?
>
> También necesitamos confirmar el modelo de asociación de múltiples emisores/tenants, autenticación y manejo de credenciales, soporte para factura y notas crédito/débito, estados y consulta/reconciliación, webhooks, idempotencia, XML y respuestas DIAN descargables, numeración/certificados, límites, SLA, soporte, precios y condiciones para comercializar la integración.
>
> Por favor indiquen la razón social y NIT de la entidad que prestará el servicio, el estado de habilitación DIAN y la versión del anexo técnico soportada. Buscamos realizar primero una prueba de sandbox; no compartiremos datos reales de clientes ni credenciales de producción en esta etapa.

Esta plantilla es solo un borrador; no se ha enviado a ningún proveedor.

## Criterios de aceptación antes de seleccionar

- Confirmación escrita de la relación plataforma–proveedor y el modelo multi-tenant.
- Pruebas de sandbox para emisión, consulta de estado, rechazo, reintento tras timeout, nota crédito y descarga de artefactos.
- Evidencia de que una retransmisión idempotente no consume otro consecutivo ni produce duplicados.
- Validación de aislamiento: un tenant no puede acceder a estado, credenciales ni artefactos fiscales de otro.
- Confirmación de soporte para los documentos y versión técnica aplicables al producto, además de requisitos de numeración y certificados.
- Evaluación legal/contable del contrato, responsabilidades, retención y tratamiento de datos.

La primera entrega técnica debe ser un informe de diligencia de DATAICO y HKA basado en esos documentos, no código de integración. Tras elegir proveedor, versionar su especificación aprobada y diseñar los mapeos antes de implementar el adaptador.

## Fuentes públicas consultadas

- [Listado DIAN de proveedores habilitados, archivo publicado como 15/10/2026](https://www.dian.gov.co/impuestos/factura-electronica/Documents/Proveedores-habilitados-15102026.xlsx).
- [DATAICO: planes y solución API](https://www.dataico.com/planes-y-precios).
- [The Factory HKA: preguntas frecuentes e integración directa](https://thefactoryhka.com.co/faqs/).
- [The Factory HKA: servicios y documentos electrónicos](https://thefactoryhka.com.co/facturacion-electronica/).
- [Alegra: documentación API](https://developer.alegra.com/llms.txt) y [términos de API](https://developer.alegra.com/docs/t%C3%A9rminos-y-condiciones-api-alegra.md).
- [Dossier Colombia y brechas actuales](COLOMBIA_E_INVOICING.md) y [ADR-011](adr/ADR-011-colombia-dian-provider-integration.md).
