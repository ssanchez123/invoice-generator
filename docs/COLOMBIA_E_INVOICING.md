# Facturación electrónica en Colombia: dossier y brechas

**Alcance:** factura electrónica de venta para Colombia, integrada mediante un proveedor tecnológico autorizado por la DIAN.
**Consulta documental:** 2026-10-21.
**Estado:** análisis de producto y arquitectura; no constituye concepto jurídico ni acredita cumplimiento tributario.

## Decisión de producto

Para el MVP, integrar un proveedor tecnológico previamente habilitado por la DIAN. El producto no buscará inicialmente la habilitación propia como proveedor tecnológico ni se presentará como “certificado por la DIAN”. La selección del proveedor concreto y la revisión del contrato, responsabilidades y condiciones de tratamiento de datos siguen pendientes.

La integración no elimina la responsabilidad tributaria del facturador electrónico. La regulación mantiene en cabeza del emisor las obligaciones sustanciales y formales, aunque contrate servicios de un proveedor tecnológico.

## Marco oficial consultado

La Resolución DIAN 227 de 2025 compila las disposiciones tributarias externas y declara que los anexos técnicos de las normas compiladas continúan vigentes. La Resolución 165 de 2023 es la fuente de las reglas de facturación electrónica y proveedores tecnológicos que se compilan allí. La DIAN también publicó modificaciones a la Resolución 165, incluidas las resoluciones 008 de 2024, 119 de 2024 y 202 de 2025; la 227 permite consultar la numeración compilada.

El micrositio técnico de la DIAN consultado lista el **Anexo Técnico de Factura Electrónica de Venta versión 1.9** y una **Caja de herramientas FE versión 1.9, v2026**. En la misma página aparecen también enlaces históricos a la versión 1.8; no deben confundirse con el enlace destacado a la versión 1.9. La documentación de DIAN enlaza además la especificación OASIS UBL 2.1.

La factura electrónica no es solamente el PDF: el flujo regulado contempla generación del documento estructurado, transmisión y validación DIAN, y entrega al adquirente. La regulación establece elementos como CUFE, QR, firma digital y documento de validación. Cuando el servicio de validación DIAN no está disponible, se aplican reglas de contingencia y transmisión posterior; el software debe modelar esa situación explícitamente, no tratarla como una validación exitosa.

### Ruta de proveedor tecnológico propio

La Resolución 165 de 2023, artículo 55 (compilado en la Resolución 227 de 2025 como artículo 1.5.1.8.1.1), contempla, entre otros, estos requisitos para solicitar y mantener la habilitación:

- Sociedad constituida en Colombia o sucursal de sociedad extranjera, inscripción en el RUT y objeto social que cubra los servicios regulados.
- Patrimonio contable mínimo de 20.000 UVT, con propiedad, planta y equipo de al menos 10.000 UVT localizado en Colombia.
- Certificación ISO 27001; la norma contempla un plazo de hasta 18 meses para aportarla tras la habilitación, pero es requisito para operar una vez vencido ese plazo.
- Plan documentado de continuidad y contingencia, infraestructura física/tecnológica y de seguridad, niveles de servicio, canal PQRSF trazable, personal con conocimientos contables, legales, UBL/XML/XSD, información de responsables y pruebas tecnológicas satisfactorias ante la DIAN.

Es una barrera de entrada material; por eso no se recomienda como primera ruta para el producto actual. Los importes y requisitos deben revisarse con asesoría tributaria y jurídica antes de cualquier decisión societaria.

## Brechas frente al repositorio

| Capacidad fiscal | Estado observado | Brecha |
|---|---|---|
| Emisor y adquirente | `Tenant` y `Customer` tienen nombre, dirección genérica y un `tax_id` sin estructura colombiana | Faltan campos fiscales tipados para NIT/identificación, dígito de verificación cuando aplique, responsabilidades/calidades y datos requeridos por la operación |
| Numeración | `Invoice.Number` es un número genérico único por tenant | No hay autorización DIAN, resolución/rango, prefijo, vigencia ni clave técnica |
| Impuestos | Líneas aceptan una tasa arbitraria en basis points | No hay clasificación fiscal colombiana, códigos/tipos de impuesto, desglose fiscal completo, exenciones/exclusiones ni retenciones |
| Cálculo | `CreateInvoiceHandler` toma `TaxRateBPS` del input | `TaxRule` es genérico y no actúa como un catálogo fiscal DIAN efectivo por fecha; el cliente de API no debe decidir unilateralmente el tratamiento tributario |
| Emisión | `IssueInvoiceHandler` cambia directamente `draft` a `issued` y persiste un snapshot | “Issued” no significa validada por DIAN. Faltan estados de envío, aceptación/rechazo, contingencia y respuesta del proveedor |
| Documento | Hay un PDF de representación genérica | No se genera XML UBL colombiano, firma/contendor, CUFE, QR ni constancia/respuesta de validación DIAN |
| Correcciones | La factura se puede cancelar mediante estado interno | No hay notas crédito/débito electrónicas, relación con el documento origen ni trazabilidad de correcciones |
| Integración | `invoice.issued` alimenta relay y servicio PDF por outbox | No existe puerto/adaptador del proveedor, credencial externa, idempotencia de transmisión, consulta de estado ni recepción de webhooks |
| Snapshot | Se captura un snapshot inmutable al emitir y el PDF usa ese snapshot | Es una buena base, pero el esquema no contiene aún todos los atributos fiscales y técnicos que exige el documento electrónico |
| Operación | Outbox tiene entregas independientes, leases y retries | Falta distinguir fallo del proveedor, rechazo fiscal, indisponibilidad DIAN y contingencia legal; reintentar una transmisión sin idempotencia externa podría duplicar documentos |

Referencias de implementación revisadas: `backend/internal/domain/entity/invoice.go`, `invoice_snapshot.go`, `customer_payment_subscription.go`, `backend/internal/application/command/create_invoice.go`, `manage_invoice.go`, `backend/internal/infrastructure/repository/invoice_repository.go`, y `backend/migrations/001_initial_schema.up.sql`.

## Dirección de arquitectura

1. Mantener el PDF informativo actual separado del artefacto fiscal validado. No modificar todavía `IssueInvoiceHandler` para afirmar que una factura está legalmente emitida.
2. Introducir un puerto de aplicación neutral al proveedor para enviar documentos, consultar su estado y recuperar los artefactos/respuestas. Las credenciales deben residir en configuración secreta administrada, nunca en `tenant.settings`, snapshots, logs o respuestas públicas.
3. Separar el ciclo comercial/contable de la factura del ciclo de cumplimiento electrónico. Persistir identificador externo, consecutivo fiscal, estado de transmisión/validación, marcas de tiempo, errores estructurados y referencias/hash de XML, respuesta DIAN y representación gráfica, con aislamiento por tenant y auditoría.
4. Usar el outbox para envío y procesamiento asíncrono, con clave idempotente estable por documento fiscal y conciliación de estados. Un retry de infraestructura no debe crear una segunda factura ni consumir un segundo consecutivo inadvertidamente.
5. Modelar rechazos y contingencias conforme al anexo DIAN y al contrato del proveedor. No convertir fallos de red en éxito; conservar evidencia y reanudar las transmisiones pendientes según el procedimiento aplicable.
6. Implementar notas crédito/débito como documentos fiscales relacionados con la factura original; no reutilizar `cancelled` como sustituto de una nota electrónica.
7. Evaluar primero un proveedor concreto con sandbox, autorización DIAN vigente, soporte de versión de anexo, callbacks/consulta de estado, idempotencia, exportación de XML y respuestas, retención, seguridad, continuidad, SLA, portabilidad y proceso de salida.

## UX y riesgos operativos del nicho

- El usuario debe distinguir claramente borrador, envío pendiente, validada, rechazada y contingencia; el estado de pago es un eje distinto.
- Tras validación, ofrecer el XML y la representación gráfica/PDF, el resultado DIAN y el identificador de consulta/QR donde corresponda.
- Rechazos deben mostrar causa accionable y preservar el documento y la evidencia originales; las correcciones se hacen mediante el documento electrónico adecuado, no editando una factura ya emitida.
- La disponibilidad del proveedor y de DIAN no es equivalente. La experiencia y los procesos deben explicar quién está indisponible, qué puede continuar y qué queda pendiente de transmisión.
- El usuario emisor sigue siendo responsable ante la DIAN. El contrato y la interfaz deben evitar sugerir que delegar en el proveedor transfiere esa responsabilidad.

## Fuentes oficiales

- [Resolución DIAN 227 de 2025 — Normograma DIAN](https://normograma.dian.gov.co/dian/compilacion/docs/resolucion_dian_0227_2025.htm): resolución única compilatoria; publicada en septiembre de 2025. Consultar especialmente el Título 5 y los artículos compilados 1.5.1.5 y 1.5.1.8.
- [Resolución DIAN 165 de 2023 — Normograma DIAN](https://normograma.dian.gov.co/dian/compilacion/docs/resolucion_dian_0165_2023.htm): fuente original; consultar artículos 1, 28, 33, 37, 55 y 56 y sus notas de compilación/modificación.
- [Documentación técnica del Sistema de Facturación Electrónica — DIAN](https://micrositios.dian.gov.co/sistema-de-facturacion-electronica/documentacion-tecnica/): enlaces publicados por DIAN al anexo 1.9 y a la caja de herramientas v2026.
- [Estatuto Tributario — Normograma DIAN](https://normograma.dian.gov.co/dian/compilacion/docs/estatuto_tributario.htm): artículos 616-1 y 616-4.
- [Decreto 1625 de 2016 — Normograma DIAN](https://normograma.dian.gov.co/dian/compilacion/docs/decreto_1625_2016.htm): reglamentación del sistema de facturación, incluidos los requisitos relacionados con proveedores tecnológicos.
- [OASIS UBL 2.1](http://docs.oasis-open.org/ubl/os-UBL-2.1/xsdrt/maindoc/): estándar enlazado desde la documentación técnica DIAN.

Antes de empezar transmisión real o declarar cumplimiento, volver a verificar en los canales DIAN la vigencia de la resolución, el anexo aplicable, la caja de pruebas y la habilitación del proveedor elegido; obtener revisión de asesoría contable/jurídica colombiana.
