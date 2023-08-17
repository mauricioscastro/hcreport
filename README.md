# hcreport
## A collection of Openshift health checks bundled in a operator 

Our CSA LATAM team has been facing the challenge of reviewing Openshift clusters, managed or not, during too many of our engangements and the current set of scripts to collect and transform data into a nice report not sparesely faces execution problems by the customers so I deciced to repack them into a operator.

This would come as a person exercise for understanding and using operators as much as getting to know Go code better. 

The general idea is to replicate all the action used to collect cluster data aggregating the result in its raw yaml form of various files as a first stage result, tranforms it using pre-configured jq and yq queries as needed into a seconday stage result also in the form of yaml files now with customized group of objects, lists and such which will be used as input data to a static site generator based on Markdown files. 

At stage 3 the static generator of your chioice (Jekyll?, MkDocs+Materials?) will use its templating language (Liquid, Jinja) to use the yaml gathered data from stage 2 as context for a static documentation site. A natural stage 4 can offer from the generated site, or automatically, the documentation converted into the pdf format (Markdown PDF, MkPDFs for MkDocs) as collection of reports.

The goal is to use Go all around and all the way so much kubectl/oc, yq and jq are going to be used as libraries for running pre-fabricated commands outside the code as configuration itens probably sourced from a yaml config file which will also bring extra freedom and power for those not familiar with go to later maintain the hcreports. Let's look at it as a toolbox operator driven by external configuration capable of extracting cluster data in a regular loop for generating nice reports. The idea is producing beautiful pdf reports for client's appreciation and guidance in respect to best practices.
