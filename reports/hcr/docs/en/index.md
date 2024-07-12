# Welcome

{{ config.site_author }}

{% for n in [1,2,3,4,5] %}
  {{ hello(config.site_author) }}
{% endfor %}

## Reported Clusters

{{ clusters }}

### SQL

```sql
{{ sql.query.example }}

```

### Links

{{ l1 }}

{{ l2 }}

!!! tip
    Good morning
!!! note
    admonition
!!! abstract
    admonition
!!! warning
    admonition    
!!! question
    admonition 
!!! example
    admonition         

## Prefácio
### Confidencialidade, direitos autorais e responsabilidade
Este documento contém informações confidenciais que são de uso exclusivo da Red Hat ©, Inc e Banco_Bradesco e não devem ser compartilhadas com pessoas que não pertençam a estas duas companhias. Este documento e quaisquer partes dele não pode ser copiado, reproduzido, fotocopiado, armazenado eletronicamente em um sistema de recuperação, ou transmitido sem o consentimento expresso por escrito da Red Hat. A Red Hat não garante que este documento esteja livre de erros ou omissões. A Red Hat Consulting se reserva o direito de fazer correções, atualizações, revisões ou alterações nas informações aqui contidas.
### Introdução
O programa Cloud Success Architect (CSA) é um programa financiado pela Red Hat que fornece um compromisso flexível e de alto nível para ajudar os clientes na adoção das tecnologias de nuvem híbrida da Red Hat. O serviço é direcionado a clientes estratégicos que adquiriram subscrições de produtos Red Hat Cloud: OpenStack, OpenShift Container Platform, Ansible, CloudForms, Azure Red Hat OpenShift [ARO], Red Hat OpenShift on Services AWS [ROSA] e OpenShift Dedicated.
### Sobre este documento
O objetivo deste documento é relatar os resultados da execução do healthcheck realizado na plataforma instalada no
ambiente Azure.
### Público
Este documento é destinado a administradores, arquitetos e desenvolvedores de sistemas do cliente: Banco_Bradesco.
### Lista de verificação de nomenclatura
As imagens a seguir auxiliarão na revisão dos pontos de controle contemplados neste documento:
### Outro Item de Teste
Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.

## Table Examples

### Table1
| Col 1 | Col 2 | Col 3 | Col 4 | Col 5 |
| - | - | - | - | - |
{% for r in queryddb(sql.query.version) -%}
| {{ r[0] }} | {{ r[1] }} | {{ r[2] }} | {{ r[3] }} | {{ r[4] }} |
{% endfor %}

### Table2
| Col 1 | Col 2 | Col 3 | Col 4 | Col 5 | Col 6 | Col 7 |
| - | - | - | - | - | - | - |
{% for r in queryddb("select * from api_resources limit 25") -%}
| {{ r[0] }} | {{ r[1] }} | {{ r[2] }} | {{ r[3] }} | {{ r[4] }} | {{ r[5] }} | {{ r[6] }} |
{% endfor %}


