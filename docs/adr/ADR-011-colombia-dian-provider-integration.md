# ADR-011: Integrar un proveedor tecnológico autorizado para Colombia

## Estado
Aceptada

## Contexto

El producto busca soportar facturación electrónica de venta en Colombia. La emisión actual es una transición de estado interna y genera un PDF genérico; no constituye envío ni validación ante la DIAN. El sistema ya tiene outbox con reintentos y un snapshot inmutable al emitir, que son bases útiles para una integración asíncrona, pero no dispone de UBL colombiano, firma, transmisión DIAN, CUFE/QR, respuestas de validación ni notas electrónicas.

La Resolución DIAN 227 de 2025 compila las normas aplicables; la DIAN publica el Anexo Técnico de Factura Electrónica de Venta versión 1.9 y una caja de herramientas v2026. Convertirse en proveedor tecnológico exige habilitación y requisitos societarios, patrimoniales, de seguridad, continuidad, personal y pruebas descritos en la regulación.

## Decisión

Para el MVP, integrar un proveedor tecnológico previamente habilitado por la DIAN. El sistema no buscará inicialmente la habilitación propia como proveedor tecnológico. La integración se mantendrá detrás de un puerto de aplicación neutral al proveedor, con adaptación en infraestructura.

La selección del proveedor y la revisión legal/contable de contrato, responsabilidades y tratamiento de datos son decisiones pendientes. La solución no se anunciará como “certificada por DIAN” sin una base normativa específica que justifique esa afirmación.

## Consecuencias

- El proveedor debe demostrar autorización DIAN vigente y compatibilidad con el anexo técnico aplicable.
- El facturador/tenant conserva sus responsabilidades tributarias; el proveedor no sustituye al emisor.
- El flujo comercial `draft/issued/paid` no se tratará como ciclo de validación fiscal. El envío, la validación y la contingencia se modelarán como estados propios.
- El outbox podrá impulsar envíos y conciliación de estado, pero cada documento requerirá una clave idempotente externa para evitar duplicados en reintentos.
- Las credenciales del proveedor se administrarán como secretos y no se guardarán en configuraciones JSON de tenant.
- La representación PDF existente seguirá siendo distinta del XML/constancia fiscal validada hasta implementar el flujo certificado.
- Notas crédito/débito, evidencia del proveedor/DIAN, respuesta a rechazos y recuperación de contingencias forman parte del alcance fiscal; cancelar una factura internamente no las reemplaza.

## Fuentes

- [Resolución DIAN 227 de 2025 — Normograma DIAN](https://normograma.dian.gov.co/dian/compilacion/docs/resolucion_dian_0227_2025.htm), Título 5, especialmente artículos compilados 1.5.1.5 y 1.5.1.8.
- [Resolución DIAN 165 de 2023 — Normograma DIAN](https://normograma.dian.gov.co/dian/compilacion/docs/resolucion_dian_0165_2023.htm), artículos 28, 37, 55 y 56.
- [Documentación técnica de factura electrónica — DIAN](https://micrositios.dian.gov.co/sistema-de-facturacion-electronica/documentacion-tecnica/).
- Brechas actuales y recomendación de arquitectura: [`docs/COLOMBIA_E_INVOICING.md`](../COLOMBIA_E_INVOICING.md).

Esta decisión técnica no constituye asesoría legal ni una declaración de cumplimiento fiscal.
