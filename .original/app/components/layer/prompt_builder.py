# DEPENDENCIES
## Local
from .injections import InjectionMap, VariableMap


def build_prompt(text_with_variables: str, injection_map: InjectionMap, variable_map: VariableMap) -> str:
    for injection_variable, injection in injection_map.items():
        injection_text: str = injection.get_injection(variable_map)
        if hasattr(injection, "sub_injection_map"):
            sub_injection_map: InjectionMap = getattr(injection, "sub_injection_map")
            if sub_injection_map:
                injection_text = build_prompt(
                    text_with_variables=injection_text,
                    injection_map=sub_injection_map,
                    variable_map=variable_map
                )
        replace_variable: str = "{{ injection_variable }}".replace("injection_variable", injection_variable)
        text_with_variables = text_with_variables.replace(replace_variable, injection_text)
    return text_with_variables
