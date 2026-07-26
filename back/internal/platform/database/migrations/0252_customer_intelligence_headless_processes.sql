-- 0252_customer_intelligence_headless_processes.sql
--
-- Publica contratos fechados para os processos headless inicialmente
-- depreciados por 0246 e registra o pipeline generico de execucao.
-- Nenhum prompt, binding, agente, source, tool ou knowledge capability e
-- publicado/habilitado por esta migration.

do $$
declare
    expected_count integer;
begin
    select count(*)
      into expected_count
      from intelligence.process_definitions
     where process_key = any (array[
        'conversation.handoff_summary',
        'memory.extract',
        'profile.summary',
        'recommendation.follow_up',
        'recommendation.offer',
        'recommendation.important_dates',
        'source.suggest',
        'portfolio.opportunity',
        'media.image_analysis',
        'media.document_analysis',
        'quality.review'
     ]::text[]);

    if expected_count <> 11 then
        raise exception
            '0252 requires all 11 canonical headless process definitions; found %',
            expected_count;
    end if;
end $$;

with headless_processes (
    process_key,
    input_schema,
    output_schema,
    schema_version,
    max_input_tokens,
    max_output_tokens,
    timeout_ms
) as (
    values
    (
        'conversation.handoff_summary'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","handoffReasonCode","conversationRef","acceptedMessageRefs","acceptedObservationIds","collectedFields","pendingFieldKeys"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["conversation.handoff_summary.input.v2"]},
            "handoffReasonCode":{"type":"string","minLength":1,"maxLength":160},
            "conversationRef":{"type":"string","minLength":1,"maxLength":160},
            "acceptedMessageRefs":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":160}},
            "acceptedObservationIds":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":64}},
            "collectedFields":{
              "type":"array",
              "maxItems":50,
              "items":{
                "type":"object",
                "required":["fieldKey","valueSummary","evidenceRefs"],
                "properties":{
                  "fieldKey":{"type":"string","minLength":1,"maxLength":160},
                  "valueSummary":{"type":"string","maxLength":1000},
                  "evidenceRefs":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}}
                },
                "additionalProperties":false
              }
            },
            "pendingFieldKeys":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["summary","reasonCode","collectedFieldKeys","pendingFieldKeys","redactionCodes","messageIds","evidenceRefs","confidence"],
          "properties":{
            "summary":{"type":"string","minLength":1,"maxLength":4000},
            "reasonCode":{"type":"string","minLength":1,"maxLength":160},
            "collectedFieldKeys":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}},
            "pendingFieldKeys":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}},
            "redactionCodes":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "messageIds":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":64}},
            "evidenceRefs":{
              "type":"array",
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["observationId","sourceKey"],
                "properties":{
                  "observationId":{"type":"string","minLength":1,"maxLength":64},
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                },
                "additionalProperties":false
              }
            },
            "confidence":{"type":"number","minimum":0,"maximum":1}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'conversation.handoff_summary.result.v2'::text,
        8000,
        1800,
        60000
    ),
    (
        'memory.extract'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","observationRefs","allowedFactKeys"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["memory.extract.input.v2"]},
            "observationRefs":{
              "type":"array",
              "minItems":1,
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["observationId","sourceKey","observedAt"],
                "properties":{
                  "observationId":{"type":"string","minLength":1,"maxLength":64},
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160},
                  "observedAt":{"type":"string","minLength":1,"maxLength":64}
                },
                "additionalProperties":false
              }
            },
            "allowedFactKeys":{"type":"array","minItems":1,"maxItems":200,"items":{"type":"string","minLength":1,"maxLength":160}}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["claims"],
          "properties":{
            "claims":{
              "type":"array",
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["factKey","valueType","value","confidence","evidenceObservationIds","validFrom","validUntil"],
                "properties":{
                  "factKey":{"type":"string","minLength":1,"maxLength":160},
                  "valueType":{"type":"string","enum":["string","integer","decimal","boolean","date","timestamp","enum","string_list","object_closed"]},
                  "value":{
                    "type":["string","integer","number","boolean","array","object","null"],
                    "maxLength":16000,
                    "maxItems":100,
                    "items":{"type":"string","maxLength":1000},
                    "maxProperties":100,
                    "additionalProperties":{"type":["string","integer","number","boolean","null"],"maxLength":4000}
                  },
                  "confidence":{"type":"number","minimum":0,"maximum":1},
                  "sensitivity":{"type":"string","enum":["public","internal","personal","sensitive","restricted"]},
                  "evidenceObservationIds":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"string","minLength":1,"maxLength":64}},
                  "validFrom":{"type":["string","null"],"maxLength":40},
                  "validUntil":{"type":["string","null"],"maxLength":40}
                },
                "additionalProperties":false
              }
            }
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'memory.extract.result.v2'::text,
        12000,
        6000,
        90000
    ),
    (
        'profile.summary'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","factRefs","conflictRefs","sectionKeys","previousSummaryVersionId"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["profile.summary.input.v2"]},
            "factRefs":{"type":"array","maxItems":200,"items":{"type":"string","minLength":1,"maxLength":160}},
            "conflictRefs":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":160}},
            "sectionKeys":{"type":"array","minItems":1,"maxItems":30,"items":{"type":"string","minLength":1,"maxLength":120}},
            "previousSummaryVersionId":{"type":["string","null"],"maxLength":64}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["summary","sections","evidenceRefs","factRefs","confidence"],
          "properties":{
            "summary":{"type":"string","minLength":1,"maxLength":8000},
            "sections":{
              "type":"array",
              "minItems":1,
              "maxItems":12,
              "items":{
                "type":"object",
                "required":["key","content","evidenceRefs","factRefs","confidence"],
                "properties":{
                  "key":{"type":"string","minLength":1,"maxLength":80},
                  "content":{"type":"string","minLength":1,"maxLength":4000},
                  "evidenceRefs":{
                    "type":"array",
                    "maxItems":100,
                    "items":{
                      "type":"object",
                      "required":["observationId","sourceKey"],
                      "properties":{
                        "observationId":{"type":"string","minLength":1,"maxLength":64},
                        "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                      },
                      "additionalProperties":false
                    }
                  },
                  "factRefs":{
                    "type":"array",
                    "maxItems":50,
                    "items":{
                      "type":"object",
                      "required":["factId","factKey","version"],
                      "properties":{
                        "factId":{"type":"string","minLength":1,"maxLength":64},
                        "factKey":{"type":"string","minLength":1,"maxLength":160},
                        "version":{"type":"integer","minimum":1,"maximum":2147483647}
                      },
                      "additionalProperties":false
                    }
                  },
                  "confidence":{"type":"number","minimum":0,"maximum":1}
                },
                "additionalProperties":false
              }
            },
            "evidenceRefs":{
              "type":"array",
              "maxItems":200,
              "items":{
                "type":"object",
                "required":["observationId","sourceKey"],
                "properties":{
                  "observationId":{"type":"string","minLength":1,"maxLength":64},
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                },
                "additionalProperties":false
              }
            },
            "factRefs":{
              "type":"array",
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["factId","factKey","version"],
                "properties":{
                  "factId":{"type":"string","minLength":1,"maxLength":64},
                  "factKey":{"type":"string","minLength":1,"maxLength":160},
                  "version":{"type":"integer","minimum":1,"maximum":2147483647}
                },
                "additionalProperties":false
              }
            },
            "confidence":{"type":"number","minimum":0,"maximum":1}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'profile.summary.result.v2'::text,
        16000,
        4000,
        90000
    ),
    (
        'recommendation.follow_up'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","profileSummaryVersionId","consentSnapshotId","cadencePolicyRef","channelEligibility","quietHours"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["recommendation.follow_up.input.v2"]},
            "profileSummaryVersionId":{"type":"string","minLength":1,"maxLength":64},
            "consentSnapshotId":{"type":"string","minLength":1,"maxLength":64},
            "cadencePolicyRef":{"type":"string","minLength":1,"maxLength":160},
            "channelEligibility":{
              "type":"array",
              "maxItems":20,
              "items":{
                "type":"object",
                "required":["channelKey","eligible","reasonCodes"],
                "properties":{
                  "channelKey":{"type":"string","minLength":1,"maxLength":120},
                  "eligible":{"type":"boolean"},
                  "reasonCodes":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}}
                },
                "additionalProperties":false
              }
            },
            "quietHours":{
              "type":"object",
              "required":["timezone","start","end"],
              "properties":{
                "timezone":{"type":"string","minLength":1,"maxLength":64},
                "start":{"type":"string","minLength":1,"maxLength":16},
                "end":{"type":"string","minLength":1,"maxLength":16}
              },
              "additionalProperties":false
            }
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["recommendedAt","windowStart","windowEnd","suggestedChannel","cadencePolicyRef","reasonCodes","conversationBrief","evidenceRefs","constraintsSnapshot","confidence","expiresAt"],
          "properties":{
            "recommendedAt":{"type":"string","minLength":1,"maxLength":40},
            "windowStart":{"type":"string","minLength":1,"maxLength":40},
            "windowEnd":{"type":"string","minLength":1,"maxLength":40},
            "suggestedChannel":{"type":"string","enum":["email","instagram","phone","sms","webchat","whatsapp"]},
            "cadencePolicyRef":{"type":"string","minLength":1,"maxLength":64},
            "reasonCodes":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "conversationBrief":{"type":"string","minLength":1,"maxLength":1000},
            "evidenceRefs":{
              "type":"array",
              "minItems":1,
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["observationId","sourceKey"],
                "properties":{
                  "observationId":{"type":"string","minLength":1,"maxLength":64},
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                },
                "additionalProperties":false
              }
            },
            "constraintsSnapshot":{
              "type":"object",
              "required":["consentEligible","channelEligible","quietHoursSatisfied","frequencyCapSatisfied","reasonCodes"],
              "properties":{
                "consentEligible":{"type":"boolean"},
                "channelEligible":{"type":"boolean"},
                "quietHoursSatisfied":{"type":"boolean"},
                "frequencyCapSatisfied":{"type":"boolean"},
                "reasonCodes":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}}
              },
              "additionalProperties":false
            },
            "confidence":{"type":"number","minimum":0,"maximum":1},
            "expiresAt":{"type":"string","minLength":1,"maxLength":40}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'recommendation.follow_up.result.v2'::text,
        12000,
        2500,
        90000
    ),
    (
        'recommendation.offer'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","profileSummaryVersionId","catalogSnapshotId","catalogOwnerModule","catalogItems","blockedItemRefs"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["recommendation.offer.input.v2"]},
            "profileSummaryVersionId":{"type":"string","minLength":1,"maxLength":64},
            "catalogSnapshotId":{"type":"string","minLength":1,"maxLength":64},
            "catalogOwnerModule":{"type":"string","minLength":1,"maxLength":120},
            "catalogItems":{
              "type":"array",
              "minItems":1,
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["itemType","itemId","versionRef"],
                "properties":{
                  "itemType":{"type":"string","minLength":1,"maxLength":120},
                  "itemId":{"type":"string","minLength":1,"maxLength":160},
                  "versionRef":{"type":"string","minLength":1,"maxLength":160}
                },
                "additionalProperties":false
              }
            },
            "blockedItemRefs":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":160}}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["catalogOwnerModule","catalogItems","fitReasonCodes","fitNarrative","excludedItemReasonCodes","priceContextRef","validityCheckedAt","evidenceRefs","factRefs","confidence","expiresAt"],
          "properties":{
            "catalogOwnerModule":{"type":"string","minLength":1,"maxLength":80},
            "catalogItems":{
              "type":"array",
              "minItems":1,
              "maxItems":20,
              "items":{
                "type":"object",
                "required":["itemType","itemId","versionRef"],
                "properties":{
                  "itemType":{"type":"string","minLength":1,"maxLength":80},
                  "itemId":{"type":"string","minLength":1,"maxLength":64},
                  "versionRef":{"type":"string","minLength":1,"maxLength":64}
                },
                "additionalProperties":false
              }
            },
            "fitReasonCodes":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "fitNarrative":{"type":"string","minLength":1,"maxLength":2000},
            "excludedItemReasonCodes":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "priceContextRef":{"type":["string","null"],"maxLength":64},
            "validityCheckedAt":{"type":"string","minLength":1,"maxLength":40},
            "evidenceRefs":{
              "type":"array",
              "minItems":1,
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["observationId","sourceKey"],
                "properties":{
                  "observationId":{"type":"string","minLength":1,"maxLength":64},
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                },
                "additionalProperties":false
              }
            },
            "factRefs":{
              "type":"array",
              "minItems":1,
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["factId","factKey","version"],
                "properties":{
                  "factId":{"type":"string","minLength":1,"maxLength":64},
                  "factKey":{"type":"string","minLength":1,"maxLength":160},
                  "version":{"type":"integer","minimum":1,"maximum":2147483647}
                },
                "additionalProperties":false
              }
            },
            "confidence":{"type":"number","minimum":0,"maximum":1},
            "expiresAt":{"type":"string","minLength":1,"maxLength":40}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'recommendation.offer.result.v2'::text,
        14000,
        3000,
        90000
    ),
    (
        'recommendation.important_dates'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","dateFactRefs","temporalEvidenceRefs","allowedDateKinds"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["recommendation.important_dates.input.v2"]},
            "dateFactRefs":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"string","minLength":1,"maxLength":160}},
            "temporalEvidenceRefs":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"string","minLength":1,"maxLength":160}},
            "allowedDateKinds":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"string","minLength":1,"maxLength":120}}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["dateFactId","dateFactVersion","dateValue","dateKind","recurrence","verificationState","suggestedWindow","reasonCodes","evidenceRefs","requiresReview","confidence","expiresAt"],
          "properties":{
            "dateFactId":{"type":"string","minLength":1,"maxLength":64},
            "dateFactVersion":{"type":"integer","minimum":1,"maximum":2147483647},
            "dateValue":{"type":"string","minLength":10,"maxLength":10},
            "dateKind":{"type":"string","minLength":1,"maxLength":80},
            "recurrence":{"type":"string","enum":["none","monthly","yearly"]},
            "verificationState":{"type":"string","enum":["verified","resolved","contested"]},
            "suggestedWindow":{
              "type":"object",
              "required":["start","end"],
              "properties":{
                "start":{"type":"string","minLength":10,"maxLength":10},
                "end":{"type":"string","minLength":10,"maxLength":10}
              },
              "additionalProperties":false
            },
            "reasonCodes":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "evidenceRefs":{
              "type":"array",
              "minItems":1,
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["observationId","sourceKey"],
                "properties":{
                  "observationId":{"type":"string","minLength":1,"maxLength":64},
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                },
                "additionalProperties":false
              }
            },
            "requiresReview":{"type":"boolean"},
            "confidence":{"type":"number","minimum":0,"maximum":1},
            "expiresAt":{"type":"string","minLength":1,"maxLength":40}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'recommendation.important_dates.result.v2'::text,
        12000,
        2500,
        90000
    ),
    (
        'source.suggest'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","gapKeys","sourceCatalog","prohibitedCapabilityKeys"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["source.suggest.input.v2"]},
            "gapKeys":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"string","minLength":1,"maxLength":160}},
            "sourceCatalog":{
              "type":"array",
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["sourceKey","capabilityKeys","freshnessState"],
                "properties":{
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160},
                  "capabilityKeys":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}},
                  "freshnessState":{"type":"string","enum":["fresh","stale","unknown","unavailable"]}
                },
                "additionalProperties":false
              }
            },
            "prohibitedCapabilityKeys":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":160}}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["suggestions"],
          "properties":{
            "suggestions":{
              "type":"array",
              "maxItems":10,
              "items":{
                "type":"object",
                "required":["sourceKey","gapCodes","rationaleCode","rationale","evidenceRefs","confidence","expiresAt"],
                "properties":{
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160},
                  "gapCodes":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
                  "rationaleCode":{"type":"string","minLength":1,"maxLength":160},
                  "rationale":{"type":"string","minLength":1,"maxLength":1000},
                  "evidenceRefs":{
                    "type":"array",
                    "maxItems":100,
                    "items":{
                      "type":"object",
                      "required":["observationId","sourceKey"],
                      "properties":{
                        "observationId":{"type":"string","minLength":1,"maxLength":64},
                        "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                      },
                      "additionalProperties":false
                    }
                  },
                  "confidence":{"type":"number","minimum":0,"maximum":1},
                  "expiresAt":{"type":"string","minLength":1,"maxLength":40}
                },
                "additionalProperties":false
              }
            }
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'source.suggest.result.v2'::text,
        8000,
        2200,
        60000
    ),
    (
        'portfolio.opportunity'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","aggregateSnapshotId","targetClientAccountIds","datasetKeys","sourceKeys","dimensionKeys","metricKeys","period","cohortClass","suppression"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["portfolio.opportunity.input.v2"]},
            "aggregateSnapshotId":{"type":"string","minLength":1,"maxLength":64},
            "targetClientAccountIds":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"string","minLength":1,"maxLength":64}},
            "datasetKeys":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}},
            "sourceKeys":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}},
            "dimensionKeys":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}},
            "metricKeys":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}},
            "period":{
              "type":"object",
              "required":["start","end"],
              "properties":{
                "start":{"type":"string","minLength":1,"maxLength":64},
                "end":{"type":"string","minLength":1,"maxLength":64}
              },
              "additionalProperties":false
            },
            "cohortClass":{"type":"string","minLength":1,"maxLength":120},
            "suppression":{
              "type":"object",
              "required":["applied","reasonCodes","policyVersionRef"],
              "properties":{
                "applied":{"type":"boolean"},
                "reasonCodes":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
                "policyVersionRef":{"type":"string","minLength":1,"maxLength":160}
              },
              "additionalProperties":false
            }
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["opportunityType","targetClientAccountIds","purposeKey","aggregateSnapshotId","datasetKeys","sourceKeys","dimensionKeys","metricKeys","period","cohortClass","cohortSize","suppressionThreshold","suppressionApplied","suppressionReasonCodes","rationale","reasonCodes","campaignBrief","policyVersionRefs","confidence","validFrom","expiresAt"],
          "properties":{
            "opportunityType":{"type":"string","minLength":1,"maxLength":80},
            "targetClientAccountIds":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":64}},
            "purposeKey":{"type":"string","enum":["portfolio_analysis"]},
            "aggregateSnapshotId":{"type":"string","minLength":1,"maxLength":64},
            "datasetKeys":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "sourceKeys":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "dimensionKeys":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "metricKeys":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "period":{
              "type":"object",
              "required":["start","end"],
              "properties":{
                "start":{"type":"string","minLength":1,"maxLength":40},
                "end":{"type":"string","minLength":1,"maxLength":40}
              },
              "additionalProperties":false
            },
            "cohortClass":{"type":"string","enum":["10_24","25_49","50_99","100_plus"]},
            "cohortSize":{"type":"integer","minimum":10,"maximum":1000000000},
            "suppressionThreshold":{"type":"integer","minimum":10,"maximum":1000000000},
            "suppressionApplied":{"type":"boolean"},
            "suppressionReasonCodes":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "rationale":{"type":"string","minLength":1,"maxLength":2000},
            "reasonCodes":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "campaignBrief":{"type":"string","minLength":1,"maxLength":2000},
            "policyVersionRefs":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string","minLength":1,"maxLength":64}},
            "confidence":{"type":"number","minimum":0,"maximum":1},
            "validFrom":{"type":"string","minLength":1,"maxLength":40},
            "expiresAt":{"type":"string","minLength":1,"maxLength":40}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'portfolio.opportunity.result.v2'::text,
        14000,
        3200,
        90000
    ),
    (
        'media.image_analysis'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","mediaAssetId","accessGrantId","mimeType","requestedFieldKeys","safetyPolicyRef"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["media.image_analysis.input.v2"]},
            "mediaAssetId":{"type":"string","minLength":1,"maxLength":160},
            "accessGrantId":{"type":"string","minLength":1,"maxLength":160},
            "mimeType":{"type":"string","enum":["image/jpeg","image/png","image/webp","image/gif","image/heic"]},
            "requestedFieldKeys":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}},
            "safetyPolicyRef":{"type":"string","minLength":1,"maxLength":160}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["description","candidateClaims","evidenceRefs","safetyFlags","blocked","blockReasonCodes","confidence"],
          "properties":{
            "description":{"type":"string","maxLength":8000},
            "candidateClaims":{
              "type":"array",
              "maxItems":50,
              "items":{
                "type":"object",
                "required":["factKey","valueType","value","confidence","evidenceObservationIds","validFrom","validUntil"],
                "properties":{
                  "factKey":{"type":"string","minLength":1,"maxLength":160},
                  "valueType":{"type":"string","enum":["string","integer","decimal","boolean","date","timestamp","enum","string_list","object_closed"]},
                  "value":{
                    "type":["string","integer","number","boolean","array","object","null"],
                    "maxLength":16000,
                    "maxItems":100,
                    "items":{"type":"string","maxLength":1000},
                    "maxProperties":100,
                    "additionalProperties":{"type":["string","integer","number","boolean","null"],"maxLength":4000}
                  },
                  "confidence":{"type":"number","minimum":0,"maximum":1},
                  "sensitivity":{"type":"string","enum":["public","internal","personal","sensitive","restricted"]},
                  "evidenceObservationIds":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"string","minLength":1,"maxLength":64}},
                  "validFrom":{"type":["string","null"],"maxLength":40},
                  "validUntil":{"type":["string","null"],"maxLength":40}
                },
                "additionalProperties":false
              }
            },
            "evidenceRefs":{
              "type":"array",
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["observationId","sourceKey"],
                "properties":{
                  "observationId":{"type":"string","minLength":1,"maxLength":64},
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                },
                "additionalProperties":false
              }
            },
            "safetyFlags":{"type":"array","maxItems":20,"items":{"type":"string","enum":["adult","financial_data","hate","identity_document","malware","medical_data","personal_data","self_harm","unknown","violence"]}},
            "blocked":{"type":"boolean"},
            "blockReasonCodes":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "confidence":{"type":"number","minimum":0,"maximum":1}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'media.image_analysis.result.v2'::text,
        12000,
        3500,
        90000
    ),
    (
        'media.document_analysis'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","mediaAssetId","accessGrantId","mimeType","pageRanges","requestedFieldKeys","chunkPolicy","safetyPolicyRef"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["media.document_analysis.input.v2"]},
            "mediaAssetId":{"type":"string","minLength":1,"maxLength":160},
            "accessGrantId":{"type":"string","minLength":1,"maxLength":160},
            "mimeType":{"type":"string","enum":["application/pdf","text/plain","text/csv","application/vnd.openxmlformats-officedocument.wordprocessingml.document"]},
            "pageRanges":{
              "type":"array",
              "maxItems":20,
              "items":{
                "type":"object",
                "required":["start","end"],
                "properties":{
                  "start":{"type":"integer","minimum":1,"maximum":10000},
                  "end":{"type":"integer","minimum":1,"maximum":10000}
                },
                "additionalProperties":false
              }
            },
            "requestedFieldKeys":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":160}},
            "chunkPolicy":{
              "type":"object",
              "required":["maxChunks","maxCharsPerChunk"],
              "properties":{
                "maxChunks":{"type":"integer","minimum":1,"maximum":100},
                "maxCharsPerChunk":{"type":"integer","minimum":128,"maximum":16000}
              },
              "additionalProperties":false
            },
            "safetyPolicyRef":{"type":"string","minLength":1,"maxLength":160}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["summary","pageCount","candidateClaims","chunks","evidenceRefs","safetyFlags","blocked","blockReasonCodes","confidence"],
          "properties":{
            "summary":{"type":"string","maxLength":8000},
            "pageCount":{"type":"integer","minimum":0,"maximum":10000},
            "candidateClaims":{
              "type":"array",
              "maxItems":50,
              "items":{
                "type":"object",
                "required":["factKey","valueType","value","confidence","evidenceObservationIds","validFrom","validUntil"],
                "properties":{
                  "factKey":{"type":"string","minLength":1,"maxLength":160},
                  "valueType":{"type":"string","enum":["string","integer","decimal","boolean","date","timestamp","enum","string_list","object_closed"]},
                  "value":{
                    "type":["string","integer","number","boolean","array","object","null"],
                    "maxLength":16000,
                    "maxItems":100,
                    "items":{"type":"string","maxLength":1000},
                    "maxProperties":100,
                    "additionalProperties":{"type":["string","integer","number","boolean","null"],"maxLength":4000}
                  },
                  "confidence":{"type":"number","minimum":0,"maximum":1},
                  "sensitivity":{"type":"string","enum":["public","internal","personal","sensitive","restricted"]},
                  "evidenceObservationIds":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"string","minLength":1,"maxLength":64}},
                  "validFrom":{"type":["string","null"],"maxLength":40},
                  "validUntil":{"type":["string","null"],"maxLength":40}
                },
                "additionalProperties":false
              }
            },
            "chunks":{
              "type":"array",
              "maxItems":20,
              "items":{
                "type":"object",
                "required":["chunkKey","pageStart","pageEnd","text","evidenceRefs"],
                "properties":{
                  "chunkKey":{"type":"string","minLength":1,"maxLength":80},
                  "pageStart":{"type":"integer","minimum":1,"maximum":10000},
                  "pageEnd":{"type":"integer","minimum":1,"maximum":10000},
                  "text":{"type":"string","minLength":1,"maxLength":4000},
                  "evidenceRefs":{
                    "type":"array",
                    "minItems":1,
                    "maxItems":100,
                    "items":{
                      "type":"object",
                      "required":["observationId","sourceKey"],
                      "properties":{
                        "observationId":{"type":"string","minLength":1,"maxLength":64},
                        "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                      },
                      "additionalProperties":false
                    }
                  }
                },
                "additionalProperties":false
              }
            },
            "evidenceRefs":{
              "type":"array",
              "maxItems":100,
              "items":{
                "type":"object",
                "required":["observationId","sourceKey"],
                "properties":{
                  "observationId":{"type":"string","minLength":1,"maxLength":64},
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                },
                "additionalProperties":false
              }
            },
            "safetyFlags":{"type":"array","maxItems":20,"items":{"type":"string","enum":["adult","financial_data","hate","identity_document","malware","medical_data","personal_data","self_harm","unknown","violence"]}},
            "blocked":{"type":"boolean"},
            "blockReasonCodes":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "confidence":{"type":"number","minimum":0,"maximum":1}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'media.document_analysis.result.v2'::text,
        24000,
        8000,
        120000
    ),
    (
        'quality.review'::text,
        $schema$
        {
          "type":"object",
          "required":["schemaVersion","interactionRef","sanitizedMessageRefs","rubricVersionRef","rubricCriteria","excludedSensitiveCategories"],
          "properties":{
            "schemaVersion":{"type":"string","enum":["quality.review.input.v2"]},
            "interactionRef":{"type":"string","minLength":1,"maxLength":160},
            "sanitizedMessageRefs":{"type":"array","minItems":1,"maxItems":100,"items":{"type":"string","minLength":1,"maxLength":160}},
            "rubricVersionRef":{"type":"string","minLength":1,"maxLength":160},
            "rubricCriteria":{
              "type":"array",
              "minItems":1,
              "maxItems":30,
              "items":{
                "type":"object",
                "required":["criterionKey","maxScore","weight"],
                "properties":{
                  "criterionKey":{"type":"string","minLength":1,"maxLength":160},
                  "maxScore":{"type":"number","exclusiveMinimum":0,"maximum":100},
                  "weight":{"type":"number","minimum":0,"maximum":1}
                },
                "additionalProperties":false
              }
            },
            "excludedSensitiveCategories":{"type":"array","maxItems":50,"items":{"type":"string","minLength":1,"maxLength":160}}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        $schema$
        {
          "type":"object",
          "required":["overallScore","scores","issues","coaching","evidenceRefs","reasonCodes","confidence"],
          "properties":{
            "overallScore":{"type":"number","minimum":0,"maximum":1},
            "scores":{
              "type":"array",
              "minItems":1,
              "maxItems":20,
              "items":{
                "type":"object",
                "required":["rubricKey","score","evidenceRefs"],
                "properties":{
                  "rubricKey":{"type":"string","minLength":1,"maxLength":80},
                  "score":{"type":"number","minimum":0,"maximum":1},
                  "evidenceRefs":{
                    "type":"array",
                    "minItems":1,
                    "maxItems":100,
                    "items":{
                      "type":"object",
                      "required":["observationId","sourceKey"],
                      "properties":{
                        "observationId":{"type":"string","minLength":1,"maxLength":64},
                        "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                      },
                      "additionalProperties":false
                    }
                  }
                },
                "additionalProperties":false
              }
            },
            "issues":{
              "type":"array",
              "maxItems":30,
              "items":{
                "type":"object",
                "required":["code","severity","description","evidenceRefs"],
                "properties":{
                  "code":{"type":"string","minLength":1,"maxLength":160},
                  "severity":{"type":"string","enum":["info","low","medium","high","critical"]},
                  "description":{"type":"string","minLength":1,"maxLength":1000},
                  "evidenceRefs":{
                    "type":"array",
                    "minItems":1,
                    "maxItems":100,
                    "items":{
                      "type":"object",
                      "required":["observationId","sourceKey"],
                      "properties":{
                        "observationId":{"type":"string","minLength":1,"maxLength":64},
                        "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                      },
                      "additionalProperties":false
                    }
                  }
                },
                "additionalProperties":false
              }
            },
            "coaching":{
              "type":"array",
              "maxItems":20,
              "items":{
                "type":"object",
                "required":["topicKey","guidance","evidenceRefs"],
                "properties":{
                  "topicKey":{"type":"string","minLength":1,"maxLength":80},
                  "guidance":{"type":"string","minLength":1,"maxLength":2000},
                  "evidenceRefs":{
                    "type":"array",
                    "minItems":1,
                    "maxItems":100,
                    "items":{
                      "type":"object",
                      "required":["observationId","sourceKey"],
                      "properties":{
                        "observationId":{"type":"string","minLength":1,"maxLength":64},
                        "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                      },
                      "additionalProperties":false
                    }
                  }
                },
                "additionalProperties":false
              }
            },
            "evidenceRefs":{
              "type":"array",
              "minItems":1,
              "maxItems":200,
              "items":{
                "type":"object",
                "required":["observationId","sourceKey"],
                "properties":{
                  "observationId":{"type":"string","minLength":1,"maxLength":64},
                  "sourceKey":{"type":"string","minLength":1,"maxLength":160}
                },
                "additionalProperties":false
              }
            },
            "reasonCodes":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":160}},
            "confidence":{"type":"number","minimum":0,"maximum":1}
          },
          "additionalProperties":false
        }
        $schema$::jsonb,
        'quality.review.result.v2'::text,
        16000,
        4500,
        90000
    )
)
insert into intelligence.process_config_versions (
    process_definition_id,
    version,
    status,
    input_schema,
    output_schema,
    schema_version,
    allowed_variables,
    allowed_source_capabilities,
    allowed_tool_capabilities,
    allowed_knowledge_capabilities,
    failure_mode,
    max_input_tokens,
    max_output_tokens,
    timeout_ms,
    published_at
)
select
    definition.id,
    2,
    'published',
    process.input_schema,
    process.output_schema,
    process.schema_version,
    '["context","input","locale","purpose","asOf"]'::jsonb,
    '[]'::jsonb,
    '[]'::jsonb,
    '[]'::jsonb,
    'no_effect',
    process.max_input_tokens,
    process.max_output_tokens,
    process.timeout_ms,
    now()
from headless_processes process
join intelligence.process_definitions definition
  on definition.process_key = process.process_key
on conflict (process_definition_id, version) do nothing;

update intelligence.process_config_versions config
   set status = 'archived'
  from intelligence.process_definitions definition
 where config.process_definition_id = definition.id
   and definition.process_key = any (array[
        'conversation.handoff_summary',
        'memory.extract',
        'profile.summary',
        'recommendation.follow_up',
        'recommendation.offer',
        'recommendation.important_dates',
        'source.suggest',
        'portfolio.opportunity',
        'media.image_analysis',
        'media.document_analysis',
        'quality.review'
   ]::text[])
   and config.version < 2
   and config.status = 'published';

update intelligence.process_definitions definition
   set status = 'registered',
       active_config_version_id = config.id,
       updated_at = now()
  from intelligence.process_config_versions config
 where config.process_definition_id = definition.id
   and config.version = 2
   and config.status = 'published'
   and config.schema_version = definition.process_key || '.result.v2'
   and definition.process_key = any (array[
        'conversation.handoff_summary',
        'memory.extract',
        'profile.summary',
        'recommendation.follow_up',
        'recommendation.offer',
        'recommendation.important_dates',
        'source.suggest',
        'portfolio.opportunity',
        'media.image_analysis',
        'media.document_analysis',
        'quality.review'
   ]::text[]);

insert into intelligence.pipeline_definitions (
    pipeline_key,
    label,
    status
)
values (
    'intelligence.headless',
    'Execucao headless de Customer Intelligence',
    'registered'
)
on conflict (pipeline_key) do update set
    label = excluded.label,
    status = 'registered',
    updated_at = now();

insert into intelligence.pipeline_versions (
    pipeline_definition_id,
    version,
    status,
    graph,
    published_at
)
select
    definition.id,
    1,
    'published',
    $graph$
    {
      "entry":"headless.process",
      "invocationMode":"headless",
      "steps":[{
        "stepKey":"headless.process",
        "kind":"process",
        "processSelection":"request.processKey",
        "allowedProcessKeys":[
          "conversation.handoff_summary",
          "memory.extract",
          "profile.summary",
          "recommendation.follow_up",
          "recommendation.offer",
          "recommendation.important_dates",
          "source.suggest",
          "portfolio.opportunity",
          "media.image_analysis",
          "media.document_analysis",
          "quality.review"
        ]
      }]
    }
    $graph$::jsonb,
    now()
from intelligence.pipeline_definitions definition
where definition.pipeline_key = 'intelligence.headless'
on conflict (pipeline_definition_id, version) do nothing;

update intelligence.pipeline_definitions definition
   set active_version_id = version.id,
       status = 'registered',
       updated_at = now()
  from intelligence.pipeline_versions version
 where version.pipeline_definition_id = definition.id
   and version.version = 1
   and version.status = 'published'
   and version.graph ->> 'invocationMode' = 'headless'
   and definition.pipeline_key = 'intelligence.headless';

do $$
declare
    active_count integer;
    contract_mismatch_count integer;
    invalid_count integer;
begin
    select count(*)
      into active_count
      from intelligence.process_definitions definition
      join intelligence.process_config_versions config
        on config.id = definition.active_config_version_id
       and config.process_definition_id = definition.id
     where definition.process_key = any (array[
        'conversation.handoff_summary',
        'memory.extract',
        'profile.summary',
        'recommendation.follow_up',
        'recommendation.offer',
        'recommendation.important_dates',
        'source.suggest',
        'portfolio.opportunity',
        'media.image_analysis',
        'media.document_analysis',
        'quality.review'
     ]::text[])
       and definition.status = 'registered'
       and config.version = 2
       and config.status = 'published'
       and config.schema_version = definition.process_key || '.result.v2';

    if active_count <> 11 then
        raise exception
            '0252 failed to activate all 11 headless process config versions; activated %',
            active_count;
    end if;

    select count(*)
      into invalid_count
      from intelligence.process_definitions definition
      join intelligence.process_config_versions config
        on config.id = definition.active_config_version_id
       and config.process_definition_id = definition.id
     where definition.process_key = any (array[
        'conversation.handoff_summary',
        'memory.extract',
        'profile.summary',
        'recommendation.follow_up',
        'recommendation.offer',
        'recommendation.important_dates',
        'source.suggest',
        'portfolio.opportunity',
        'media.image_analysis',
        'media.document_analysis',
        'quality.review'
     ]::text[])
       and (
            config.input_schema ->> 'type' <> 'object'
         or config.input_schema -> 'additionalProperties' <> 'false'::jsonb
         or config.output_schema ->> 'type' <> 'object'
         or config.output_schema -> 'additionalProperties' <> 'false'::jsonb
         or jsonb_array_length(config.allowed_source_capabilities) <> 0
         or jsonb_array_length(config.allowed_tool_capabilities) <> 0
         or jsonb_array_length(config.allowed_knowledge_capabilities) <> 0
       );

    if invalid_count <> 0 then
        raise exception
            '0252 found % active headless configs with an open root schema or implicit capability',
            invalid_count;
    end if;

    with expected_contracts(process_key, output_keys) as (
        values
        (
            'conversation.handoff_summary'::text,
            array[
                'collectedFieldKeys', 'confidence', 'evidenceRefs', 'messageIds',
                'pendingFieldKeys', 'reasonCode', 'redactionCodes', 'summary'
            ]::text[]
        ),
        ('memory.extract', array['claims']::text[]),
        (
            'profile.summary',
            array['confidence', 'evidenceRefs', 'factRefs', 'sections', 'summary']::text[]
        ),
        (
            'recommendation.follow_up',
            array[
                'cadencePolicyRef', 'confidence', 'constraintsSnapshot',
                'conversationBrief', 'evidenceRefs', 'expiresAt', 'reasonCodes',
                'recommendedAt', 'suggestedChannel', 'windowEnd', 'windowStart'
            ]::text[]
        ),
        (
            'recommendation.offer',
            array[
                'catalogItems', 'catalogOwnerModule', 'confidence', 'evidenceRefs',
                'excludedItemReasonCodes', 'expiresAt', 'factRefs', 'fitNarrative',
                'fitReasonCodes', 'priceContextRef', 'validityCheckedAt'
            ]::text[]
        ),
        (
            'recommendation.important_dates',
            array[
                'confidence', 'dateFactId', 'dateFactVersion', 'dateKind',
                'dateValue', 'evidenceRefs', 'expiresAt', 'reasonCodes',
                'recurrence', 'requiresReview', 'suggestedWindow',
                'verificationState'
            ]::text[]
        ),
        ('source.suggest', array['suggestions']::text[]),
        (
            'portfolio.opportunity',
            array[
                'aggregateSnapshotId', 'campaignBrief', 'cohortClass', 'cohortSize',
                'confidence', 'datasetKeys', 'dimensionKeys', 'expiresAt',
                'metricKeys', 'opportunityType', 'period', 'policyVersionRefs',
                'purposeKey', 'rationale', 'reasonCodes', 'sourceKeys',
                'suppressionApplied', 'suppressionReasonCodes',
                'suppressionThreshold', 'targetClientAccountIds', 'validFrom'
            ]::text[]
        ),
        (
            'media.image_analysis',
            array[
                'blockReasonCodes', 'blocked', 'candidateClaims', 'confidence',
                'description', 'evidenceRefs', 'safetyFlags'
            ]::text[]
        ),
        (
            'media.document_analysis',
            array[
                'blockReasonCodes', 'blocked', 'candidateClaims', 'chunks',
                'confidence', 'evidenceRefs', 'pageCount', 'safetyFlags', 'summary'
            ]::text[]
        ),
        (
            'quality.review',
            array[
                'coaching', 'confidence', 'evidenceRefs', 'issues',
                'overallScore', 'reasonCodes', 'scores'
            ]::text[]
        )
    )
    select count(*)
      into contract_mismatch_count
      from expected_contracts expected
      join intelligence.process_definitions definition
        on definition.process_key = expected.process_key
      join intelligence.process_config_versions config
        on config.id = definition.active_config_version_id
       and config.process_definition_id = definition.id
     where array(
               select required_key
                 from jsonb_array_elements_text(config.output_schema -> 'required')
                      required_key
                order by required_key
           ) <> array(
               select expected_key
                 from unnest(expected.output_keys) expected_key
                order by expected_key
           )
        or array(
               select key
                 from jsonb_object_keys(config.output_schema -> 'properties') key
                order by key
           ) <> array(
               select expected_key
                 from unnest(expected.output_keys) expected_key
                order by expected_key
           );

    if contract_mismatch_count <> 0 then
        raise exception
            '0252 found % output contracts incompatible with typed Go results',
            contract_mismatch_count;
    end if;

    if not exists (
        select 1
          from intelligence.pipeline_definitions definition
          join intelligence.pipeline_versions version
            on version.id = definition.active_version_id
           and version.pipeline_definition_id = definition.id
         where definition.pipeline_key = 'intelligence.headless'
           and definition.status = 'registered'
           and version.version = 1
           and version.status = 'published'
           and version.graph ->> 'invocationMode' = 'headless'
    ) then
        raise exception
            '0252 failed to publish the intelligence.headless pipeline';
    end if;
end $$;
