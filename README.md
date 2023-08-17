# hcreport
## A collection of Openshift health checks bundled in a operator 

Our CSA LATAM team has been facing the challenge of reviewing Openshift clusters, managed or not, during too many of our engangements and the current set of scripts to collect and transform data into a nice report not sparesely faces execution problems by the customers I have deciced to repack them into a operator.

This would come as a person exercise for understanding and using operators as muuch as getting to know Go code better. 

The general idea is to replciate all the action used to collect cluster data aggregating the result, as a first stage result, in its raw yaml form of various files, tranforms it using jq and yq as needed into a seconday stage result also in the form of yaml files now with customized group of objects, lists and such that will be used as input data to a static site generator based on Markdown files. 

At stage 3 the static generator of your chioice (Jekyll?, MkDocs Materials?) will use its templating language (Liquid, Jinja) to use the yaml gathered data from stage 2 as context for a static documentation site. A natural stage 4 can offer from the generated site, or automatically, the documentation converted into the pdf format (Markdown PDF, MkPDFs for MkDocs) as collection of reports.

