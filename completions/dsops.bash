# bash completion for dsops                                -*- shell-script -*-

__dsops_debug()
{
    if [[ -n ${BASH_COMP_DEBUG_FILE:-} ]]; then
        echo "$*" >> "${BASH_COMP_DEBUG_FILE}"
    fi
}

# Homebrew on Macs have version 1.3 of bash-completion which doesn't include
# _init_completion. This is a very minimal version of that function.
__dsops_init_completion()
{
    COMPREPLY=()
    _get_comp_words_by_ref "$@" cur prev words cword
}

__dsops_index_of_word()
{
    local w word=$1
    shift
    index=0
    for w in "$@"; do
        [[ $w = "$word" ]] && return
        index=$((index+1))
    done
    index=-1
}

__dsops_contains_word()
{
    local w word=$1; shift
    for w in "$@"; do
        [[ $w = "$word" ]] && return
    done
    return 1
}

__dsops_handle_go_custom_completion()
{
    __dsops_debug "${FUNCNAME[0]}: cur is ${cur}, words[*] is ${words[*]}, #words[@] is ${#words[@]}"

    local shellCompDirectiveError=1
    local shellCompDirectiveNoSpace=2
    local shellCompDirectiveNoFileComp=4
    local shellCompDirectiveFilterFileExt=8
    local shellCompDirectiveFilterDirs=16

    local out requestComp lastParam lastChar comp directive args

    # Prepare the command to request completions for the program.
    # Calling ${words[0]} instead of directly dsops allows handling aliases
    args=("${words[@]:1}")
    # Disable ActiveHelp which is not supported for bash completion v1
    requestComp="DSOPS_ACTIVE_HELP=0 ${words[0]} __completeNoDesc ${args[*]}"

    lastParam=${words[$((${#words[@]}-1))]}
    lastChar=${lastParam:$((${#lastParam}-1)):1}
    __dsops_debug "${FUNCNAME[0]}: lastParam ${lastParam}, lastChar ${lastChar}"

    if [ -z "${cur}" ] && [ "${lastChar}" != "=" ]; then
        # If the last parameter is complete (there is a space following it)
        # We add an extra empty parameter so we can indicate this to the go method.
        __dsops_debug "${FUNCNAME[0]}: Adding extra empty parameter"
        requestComp="${requestComp} \"\""
    fi

    __dsops_debug "${FUNCNAME[0]}: calling ${requestComp}"
    # Use eval to handle any environment variables and such
    out=$(eval "${requestComp}" 2>/dev/null)

    # Extract the directive integer at the very end of the output following a colon (:)
    directive=${out##*:}
    # Remove the directive
    out=${out%:*}
    if [ "${directive}" = "${out}" ]; then
        # There is not directive specified
        directive=0
    fi
    __dsops_debug "${FUNCNAME[0]}: the completion directive is: ${directive}"
    __dsops_debug "${FUNCNAME[0]}: the completions are: ${out}"

    if [ $((directive & shellCompDirectiveError)) -ne 0 ]; then
        # Error code.  No completion.
        __dsops_debug "${FUNCNAME[0]}: received error from custom completion go code"
        return
    else
        if [ $((directive & shellCompDirectiveNoSpace)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __dsops_debug "${FUNCNAME[0]}: activating no space"
                compopt -o nospace
            fi
        fi
        if [ $((directive & shellCompDirectiveNoFileComp)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __dsops_debug "${FUNCNAME[0]}: activating no file completion"
                compopt +o default
            fi
        fi
    fi

    if [ $((directive & shellCompDirectiveFilterFileExt)) -ne 0 ]; then
        # File extension filtering
        local fullFilter filter filteringCmd
        # Do not use quotes around the $out variable or else newline
        # characters will be kept.
        for filter in ${out}; do
            fullFilter+="$filter|"
        done

        filteringCmd="_filedir $fullFilter"
        __dsops_debug "File filtering command: $filteringCmd"
        $filteringCmd
    elif [ $((directive & shellCompDirectiveFilterDirs)) -ne 0 ]; then
        # File completion for directories only
        local subdir
        # Use printf to strip any trailing newline
        subdir=$(printf "%s" "${out}")
        if [ -n "$subdir" ]; then
            __dsops_debug "Listing directories in $subdir"
            __dsops_handle_subdirs_in_dir_flag "$subdir"
        else
            __dsops_debug "Listing directories in ."
            _filedir -d
        fi
    else
        while IFS='' read -r comp; do
            COMPREPLY+=("$comp")
        done < <(compgen -W "${out}" -- "$cur")
    fi
}

__dsops_handle_reply()
{
    __dsops_debug "${FUNCNAME[0]}"
    local comp
    case $cur in
        -*)
            if [[ $(type -t compopt) = "builtin" ]]; then
                compopt -o nospace
            fi
            local allflags
            if [ ${#must_have_one_flag[@]} -ne 0 ]; then
                allflags=("${must_have_one_flag[@]}")
            else
                allflags=("${flags[*]} ${two_word_flags[*]}")
            fi
            while IFS='' read -r comp; do
                COMPREPLY+=("$comp")
            done < <(compgen -W "${allflags[*]}" -- "$cur")
            if [[ $(type -t compopt) = "builtin" ]]; then
                [[ "${COMPREPLY[0]}" == *= ]] || compopt +o nospace
            fi

            # complete after --flag=abc
            if [[ $cur == *=* ]]; then
                if [[ $(type -t compopt) = "builtin" ]]; then
                    compopt +o nospace
                fi

                local index flag
                flag="${cur%=*}"
                __dsops_index_of_word "${flag}" "${flags_with_completion[@]}"
                COMPREPLY=()
                if [[ ${index} -ge 0 ]]; then
                    PREFIX=""
                    cur="${cur#*=}"
                    ${flags_completion[${index}]}
                    if [ -n "${ZSH_VERSION:-}" ]; then
                        # zsh completion needs --flag= prefix
                        eval "COMPREPLY=( \"\${COMPREPLY[@]/#/${flag}=}\" )"
                    fi
                fi
            fi

            if [[ -z "${flag_parsing_disabled}" ]]; then
                # If flag parsing is enabled, we have completed the flags and can return.
                # If flag parsing is disabled, we may not know all (or any) of the flags, so we fallthrough
                # to possibly call handle_go_custom_completion.
                return 0;
            fi
            ;;
    esac

    # check if we are handling a flag with special work handling
    local index
    __dsops_index_of_word "${prev}" "${flags_with_completion[@]}"
    if [[ ${index} -ge 0 ]]; then
        ${flags_completion[${index}]}
        return
    fi

    # we are parsing a flag and don't have a special handler, no completion
    if [[ ${cur} != "${words[cword]}" ]]; then
        return
    fi

    local completions
    completions=("${commands[@]}")
    if [[ ${#must_have_one_noun[@]} -ne 0 ]]; then
        completions+=("${must_have_one_noun[@]}")
    elif [[ -n "${has_completion_function}" ]]; then
        # if a go completion function is provided, defer to that function
        __dsops_handle_go_custom_completion
    fi
    if [[ ${#must_have_one_flag[@]} -ne 0 ]]; then
        completions+=("${must_have_one_flag[@]}")
    fi
    while IFS='' read -r comp; do
        COMPREPLY+=("$comp")
    done < <(compgen -W "${completions[*]}" -- "$cur")

    if [[ ${#COMPREPLY[@]} -eq 0 && ${#noun_aliases[@]} -gt 0 && ${#must_have_one_noun[@]} -ne 0 ]]; then
        while IFS='' read -r comp; do
            COMPREPLY+=("$comp")
        done < <(compgen -W "${noun_aliases[*]}" -- "$cur")
    fi

    if [[ ${#COMPREPLY[@]} -eq 0 ]]; then
        if declare -F __dsops_custom_func >/dev/null; then
            # try command name qualified custom func
            __dsops_custom_func
        else
            # otherwise fall back to unqualified for compatibility
            declare -F __custom_func >/dev/null && __custom_func
        fi
    fi

    # available in bash-completion >= 2, not always present on macOS
    if declare -F __ltrim_colon_completions >/dev/null; then
        __ltrim_colon_completions "$cur"
    fi

    # If there is only 1 completion and it is a flag with an = it will be completed
    # but we don't want a space after the =
    if [[ "${#COMPREPLY[@]}" -eq "1" ]] && [[ $(type -t compopt) = "builtin" ]] && [[ "${COMPREPLY[0]}" == --*= ]]; then
       compopt -o nospace
    fi
}

# The arguments should be in the form "ext1|ext2|extn"
__dsops_handle_filename_extension_flag()
{
    local ext="$1"
    _filedir "@(${ext})"
}

__dsops_handle_subdirs_in_dir_flag()
{
    local dir="$1"
    pushd "${dir}" >/dev/null 2>&1 && _filedir -d && popd >/dev/null 2>&1 || return
}

__dsops_handle_flag()
{
    __dsops_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    # if a command required a flag, and we found it, unset must_have_one_flag()
    local flagname=${words[c]}
    local flagvalue=""
    # if the word contained an =
    if [[ ${words[c]} == *"="* ]]; then
        flagvalue=${flagname#*=} # take in as flagvalue after the =
        flagname=${flagname%=*} # strip everything after the =
        flagname="${flagname}=" # but put the = back
    fi
    __dsops_debug "${FUNCNAME[0]}: looking for ${flagname}"
    if __dsops_contains_word "${flagname}" "${must_have_one_flag[@]}"; then
        must_have_one_flag=()
    fi

    # if you set a flag which only applies to this command, don't show subcommands
    if __dsops_contains_word "${flagname}" "${local_nonpersistent_flags[@]}"; then
      commands=()
    fi

    # keep flag value with flagname as flaghash
    # flaghash variable is an associative array which is only supported in bash > 3.
    if [[ -z "${BASH_VERSION:-}" || "${BASH_VERSINFO[0]:-}" -gt 3 ]]; then
        if [ -n "${flagvalue}" ] ; then
            flaghash[${flagname}]=${flagvalue}
        elif [ -n "${words[ $((c+1)) ]}" ] ; then
            flaghash[${flagname}]=${words[ $((c+1)) ]}
        else
            flaghash[${flagname}]="true" # pad "true" for bool flag
        fi
    fi

    # skip the argument to a two word flag
    if [[ ${words[c]} != *"="* ]] && __dsops_contains_word "${words[c]}" "${two_word_flags[@]}"; then
        __dsops_debug "${FUNCNAME[0]}: found a flag ${words[c]}, skip the next argument"
        c=$((c+1))
        # if we are looking for a flags value, don't show commands
        if [[ $c -eq $cword ]]; then
            commands=()
        fi
    fi

    c=$((c+1))

}

__dsops_handle_noun()
{
    __dsops_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    if __dsops_contains_word "${words[c]}" "${must_have_one_noun[@]}"; then
        must_have_one_noun=()
    elif __dsops_contains_word "${words[c]}" "${noun_aliases[@]}"; then
        must_have_one_noun=()
    fi

    nouns+=("${words[c]}")
    c=$((c+1))
}

__dsops_handle_command()
{
    __dsops_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"

    local next_command
    if [[ -n ${last_command} ]]; then
        next_command="_${last_command}_${words[c]//:/__}"
    else
        if [[ $c -eq 0 ]]; then
            next_command="_dsops_root_command"
        else
            next_command="_${words[c]//:/__}"
        fi
    fi
    c=$((c+1))
    __dsops_debug "${FUNCNAME[0]}: looking for ${next_command}"
    declare -F "$next_command" >/dev/null && $next_command
}

__dsops_handle_word()
{
    if [[ $c -ge $cword ]]; then
        __dsops_handle_reply
        return
    fi
    __dsops_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    if [[ "${words[c]}" == -* ]]; then
        __dsops_handle_flag
    elif __dsops_contains_word "${words[c]}" "${commands[@]}"; then
        __dsops_handle_command
    elif [[ $c -eq 0 ]]; then
        __dsops_handle_command
    elif __dsops_contains_word "${words[c]}" "${command_aliases[@]}"; then
        # aliashash variable is an associative array which is only supported in bash > 3.
        if [[ -z "${BASH_VERSION:-}" || "${BASH_VERSINFO[0]:-}" -gt 3 ]]; then
            words[c]=${aliashash[${words[c]}]}
            __dsops_handle_command
        else
            __dsops_handle_noun
        fi
    else
        __dsops_handle_noun
    fi
    __dsops_handle_word
}

_dsops_completion()
{
    last_command="dsops_completion"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--help")
    flags+=("-h")
    local_nonpersistent_flags+=("--help")
    local_nonpersistent_flags+=("-h")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    must_have_one_noun+=("bash")
    must_have_one_noun+=("fish")
    must_have_one_noun+=("powershell")
    must_have_one_noun+=("zsh")
    noun_aliases=()
}

_dsops_doctor()
{
    last_command="dsops_doctor"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--data-dir=")
    two_word_flags+=("--data-dir")
    local_nonpersistent_flags+=("--data-dir")
    local_nonpersistent_flags+=("--data-dir=")
    flags+=("--env=")
    two_word_flags+=("--env")
    local_nonpersistent_flags+=("--env")
    local_nonpersistent_flags+=("--env=")
    flags+=("--verbose")
    local_nonpersistent_flags+=("--verbose")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_exec()
{
    last_command="dsops_exec"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--allow-override")
    local_nonpersistent_flags+=("--allow-override")
    flags+=("--env=")
    two_word_flags+=("--env")
    local_nonpersistent_flags+=("--env")
    local_nonpersistent_flags+=("--env=")
    flags+=("--print")
    local_nonpersistent_flags+=("--print")
    flags+=("--timeout=")
    two_word_flags+=("--timeout")
    local_nonpersistent_flags+=("--timeout")
    local_nonpersistent_flags+=("--timeout=")
    flags+=("--working-dir=")
    two_word_flags+=("--working-dir")
    local_nonpersistent_flags+=("--working-dir")
    local_nonpersistent_flags+=("--working-dir=")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_flag+=("--env=")
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_get()
{
    last_command="dsops_get"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--env=")
    two_word_flags+=("--env")
    local_nonpersistent_flags+=("--env")
    local_nonpersistent_flags+=("--env=")
    flags+=("--json")
    local_nonpersistent_flags+=("--json")
    flags+=("--var=")
    two_word_flags+=("--var")
    local_nonpersistent_flags+=("--var")
    local_nonpersistent_flags+=("--var=")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_flag+=("--env=")
    must_have_one_flag+=("--var=")
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_guard_gitignore()
{
    last_command="dsops_guard_gitignore"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--path=")
    two_word_flags+=("--path")
    local_nonpersistent_flags+=("--path")
    local_nonpersistent_flags+=("--path=")
    flags+=("--verbose")
    flags+=("-v")
    local_nonpersistent_flags+=("--verbose")
    local_nonpersistent_flags+=("-v")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_guard_repo()
{
    last_command="dsops_guard_repo"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--all")
    local_nonpersistent_flags+=("--all")
    flags+=("--exit-code")
    local_nonpersistent_flags+=("--exit-code")
    flags+=("--path=")
    two_word_flags+=("--path")
    local_nonpersistent_flags+=("--path")
    local_nonpersistent_flags+=("--path=")
    flags+=("--pattern=")
    two_word_flags+=("--pattern")
    local_nonpersistent_flags+=("--pattern")
    local_nonpersistent_flags+=("--pattern=")
    flags+=("--verbose")
    flags+=("-v")
    local_nonpersistent_flags+=("--verbose")
    local_nonpersistent_flags+=("-v")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_guard()
{
    last_command="dsops_guard"

    command_aliases=()

    commands=()
    commands+=("gitignore")
    commands+=("repo")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_help()
{
    last_command="dsops_help"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    has_completion_function=1
    noun_aliases=()
}

_dsops_init()
{
    last_command="dsops_init"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--example=")
    two_word_flags+=("--example")
    local_nonpersistent_flags+=("--example")
    local_nonpersistent_flags+=("--example=")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_install-hook()
{
    last_command="dsops_install-hook"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--force")
    flags+=("-f")
    local_nonpersistent_flags+=("--force")
    local_nonpersistent_flags+=("-f")
    flags+=("--path=")
    two_word_flags+=("--path")
    local_nonpersistent_flags+=("--path")
    local_nonpersistent_flags+=("--path=")
    flags+=("--uninstall")
    local_nonpersistent_flags+=("--uninstall")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_leak_list()
{
    last_command="dsops_leak_list"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--all")
    flags+=("-a")
    local_nonpersistent_flags+=("--all")
    local_nonpersistent_flags+=("-a")
    flags+=("--status=")
    two_word_flags+=("--status")
    local_nonpersistent_flags+=("--status")
    local_nonpersistent_flags+=("--status=")
    flags+=("--type=")
    two_word_flags+=("--type")
    local_nonpersistent_flags+=("--type")
    local_nonpersistent_flags+=("--type=")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_leak_report()
{
    last_command="dsops_leak_report"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--commit=")
    two_word_flags+=("--commit")
    local_nonpersistent_flags+=("--commit")
    local_nonpersistent_flags+=("--commit=")
    flags+=("--description=")
    two_word_flags+=("--description")
    two_word_flags+=("-d")
    local_nonpersistent_flags+=("--description")
    local_nonpersistent_flags+=("--description=")
    local_nonpersistent_flags+=("-d")
    flags+=("--file=")
    two_word_flags+=("--file")
    local_nonpersistent_flags+=("--file")
    local_nonpersistent_flags+=("--file=")
    flags+=("--notify")
    flags+=("-n")
    local_nonpersistent_flags+=("--notify")
    local_nonpersistent_flags+=("-n")
    flags+=("--secret=")
    two_word_flags+=("--secret")
    local_nonpersistent_flags+=("--secret")
    local_nonpersistent_flags+=("--secret=")
    flags+=("--severity=")
    two_word_flags+=("--severity")
    two_word_flags+=("-s")
    local_nonpersistent_flags+=("--severity")
    local_nonpersistent_flags+=("--severity=")
    local_nonpersistent_flags+=("-s")
    flags+=("--title=")
    two_word_flags+=("--title")
    local_nonpersistent_flags+=("--title")
    local_nonpersistent_flags+=("--title=")
    flags+=("--type=")
    two_word_flags+=("--type")
    two_word_flags+=("-t")
    local_nonpersistent_flags+=("--type")
    local_nonpersistent_flags+=("--type=")
    local_nonpersistent_flags+=("-t")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_leak_resolve()
{
    last_command="dsops_leak_resolve"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--notes=")
    two_word_flags+=("--notes")
    two_word_flags+=("-n")
    local_nonpersistent_flags+=("--notes")
    local_nonpersistent_flags+=("--notes=")
    local_nonpersistent_flags+=("-n")
    flags+=("--notify")
    local_nonpersistent_flags+=("--notify")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_leak_show()
{
    last_command="dsops_leak_show"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_leak_update()
{
    last_command="dsops_leak_update"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--action=")
    two_word_flags+=("--action")
    local_nonpersistent_flags+=("--action")
    local_nonpersistent_flags+=("--action=")
    flags+=("--commit=")
    two_word_flags+=("--commit")
    local_nonpersistent_flags+=("--commit")
    local_nonpersistent_flags+=("--commit=")
    flags+=("--file=")
    two_word_flags+=("--file")
    local_nonpersistent_flags+=("--file")
    local_nonpersistent_flags+=("--file=")
    flags+=("--notify")
    flags+=("-n")
    local_nonpersistent_flags+=("--notify")
    local_nonpersistent_flags+=("-n")
    flags+=("--secret=")
    two_word_flags+=("--secret")
    local_nonpersistent_flags+=("--secret")
    local_nonpersistent_flags+=("--secret=")
    flags+=("--status=")
    two_word_flags+=("--status")
    two_word_flags+=("-s")
    local_nonpersistent_flags+=("--status")
    local_nonpersistent_flags+=("--status=")
    local_nonpersistent_flags+=("-s")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_leak()
{
    last_command="dsops_leak"

    command_aliases=()

    commands=()
    commands+=("list")
    commands+=("report")
    commands+=("resolve")
    commands+=("show")
    commands+=("update")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_login()
{
    last_command="dsops_login"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--interactive")
    flags+=("-i")
    local_nonpersistent_flags+=("--interactive")
    local_nonpersistent_flags+=("-i")
    flags+=("--list")
    flags+=("-l")
    local_nonpersistent_flags+=("--list")
    local_nonpersistent_flags+=("-l")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_plan()
{
    last_command="dsops_plan"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--data-dir=")
    two_word_flags+=("--data-dir")
    local_nonpersistent_flags+=("--data-dir")
    local_nonpersistent_flags+=("--data-dir=")
    flags+=("--env=")
    two_word_flags+=("--env")
    local_nonpersistent_flags+=("--env")
    local_nonpersistent_flags+=("--env=")
    flags+=("--json")
    local_nonpersistent_flags+=("--json")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_flag+=("--env=")
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_providers()
{
    last_command="dsops_providers"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--verbose")
    local_nonpersistent_flags+=("--verbose")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_render()
{
    last_command="dsops_render"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--env=")
    two_word_flags+=("--env")
    local_nonpersistent_flags+=("--env")
    local_nonpersistent_flags+=("--env=")
    flags+=("--format=")
    two_word_flags+=("--format")
    local_nonpersistent_flags+=("--format")
    local_nonpersistent_flags+=("--format=")
    flags+=("--out=")
    two_word_flags+=("--out")
    local_nonpersistent_flags+=("--out")
    local_nonpersistent_flags+=("--out=")
    flags+=("--permissions=")
    two_word_flags+=("--permissions")
    local_nonpersistent_flags+=("--permissions")
    local_nonpersistent_flags+=("--permissions=")
    flags+=("--template=")
    two_word_flags+=("--template")
    local_nonpersistent_flags+=("--template")
    local_nonpersistent_flags+=("--template=")
    flags+=("--ttl=")
    two_word_flags+=("--ttl")
    local_nonpersistent_flags+=("--ttl")
    local_nonpersistent_flags+=("--ttl=")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_flag+=("--env=")
    must_have_one_flag+=("--out=")
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_rotation_history()
{
    last_command="dsops_rotation_history"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--format=")
    two_word_flags+=("--format")
    local_nonpersistent_flags+=("--format")
    local_nonpersistent_flags+=("--format=")
    flags+=("--limit=")
    two_word_flags+=("--limit")
    local_nonpersistent_flags+=("--limit")
    local_nonpersistent_flags+=("--limit=")
    flags+=("--since=")
    two_word_flags+=("--since")
    local_nonpersistent_flags+=("--since")
    local_nonpersistent_flags+=("--since=")
    flags+=("--status=")
    two_word_flags+=("--status")
    local_nonpersistent_flags+=("--status")
    local_nonpersistent_flags+=("--status=")
    flags+=("--until=")
    two_word_flags+=("--until")
    local_nonpersistent_flags+=("--until")
    local_nonpersistent_flags+=("--until=")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_rotation_rollback()
{
    last_command="dsops_rotation_rollback"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--dry-run")
    local_nonpersistent_flags+=("--dry-run")
    flags+=("--env=")
    two_word_flags+=("--env")
    local_nonpersistent_flags+=("--env")
    local_nonpersistent_flags+=("--env=")
    flags+=("--force")
    flags+=("-f")
    local_nonpersistent_flags+=("--force")
    local_nonpersistent_flags+=("-f")
    flags+=("--reason=")
    two_word_flags+=("--reason")
    local_nonpersistent_flags+=("--reason")
    local_nonpersistent_flags+=("--reason=")
    flags+=("--service=")
    two_word_flags+=("--service")
    local_nonpersistent_flags+=("--service")
    local_nonpersistent_flags+=("--service=")
    flags+=("--verbose")
    flags+=("-v")
    local_nonpersistent_flags+=("--verbose")
    local_nonpersistent_flags+=("-v")
    flags+=("--version=")
    two_word_flags+=("--version")
    local_nonpersistent_flags+=("--version")
    local_nonpersistent_flags+=("--version=")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_flag+=("--env=")
    must_have_one_flag+=("--reason=")
    must_have_one_flag+=("--service=")
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_rotation_status()
{
    last_command="dsops_rotation_status"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--format=")
    two_word_flags+=("--format")
    local_nonpersistent_flags+=("--format")
    local_nonpersistent_flags+=("--format=")
    flags+=("--verbose")
    flags+=("-v")
    local_nonpersistent_flags+=("--verbose")
    local_nonpersistent_flags+=("-v")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_rotation()
{
    last_command="dsops_rotation"

    command_aliases=()

    commands=()
    commands+=("history")
    commands+=("rollback")
    commands+=("status")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_secrets_history()
{
    last_command="dsops_secrets_history"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_secrets_rotate()
{
    last_command="dsops_secrets_rotate"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--dry-run")
    local_nonpersistent_flags+=("--dry-run")
    flags+=("--env=")
    two_word_flags+=("--env")
    local_nonpersistent_flags+=("--env")
    local_nonpersistent_flags+=("--env=")
    flags+=("--force")
    local_nonpersistent_flags+=("--force")
    flags+=("--key=")
    two_word_flags+=("--key")
    local_nonpersistent_flags+=("--key")
    local_nonpersistent_flags+=("--key=")
    flags+=("--new-value=")
    two_word_flags+=("--new-value")
    local_nonpersistent_flags+=("--new-value")
    local_nonpersistent_flags+=("--new-value=")
    flags+=("--notify=")
    two_word_flags+=("--notify")
    local_nonpersistent_flags+=("--notify")
    local_nonpersistent_flags+=("--notify=")
    flags+=("--on-conflict=")
    two_word_flags+=("--on-conflict")
    local_nonpersistent_flags+=("--on-conflict")
    local_nonpersistent_flags+=("--on-conflict=")
    flags+=("--strategy=")
    two_word_flags+=("--strategy")
    local_nonpersistent_flags+=("--strategy")
    local_nonpersistent_flags+=("--strategy=")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_flag+=("--env=")
    must_have_one_flag+=("--key=")
    must_have_one_flag+=("--strategy=")
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_secrets_status()
{
    last_command="dsops_secrets_status"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_secrets()
{
    last_command="dsops_secrets"

    command_aliases=()

    commands=()
    commands+=("history")
    commands+=("rotate")
    commands+=("status")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_shred()
{
    last_command="dsops_shred"

    command_aliases=()

    commands=()

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--force")
    flags+=("-f")
    local_nonpersistent_flags+=("--force")
    local_nonpersistent_flags+=("-f")
    flags+=("--passes=")
    two_word_flags+=("--passes")
    two_word_flags+=("-n")
    local_nonpersistent_flags+=("--passes")
    local_nonpersistent_flags+=("--passes=")
    local_nonpersistent_flags+=("-n")
    flags+=("--recursive")
    flags+=("-r")
    local_nonpersistent_flags+=("--recursive")
    local_nonpersistent_flags+=("-r")
    flags+=("--verbose")
    flags+=("-v")
    local_nonpersistent_flags+=("--verbose")
    local_nonpersistent_flags+=("-v")
    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

_dsops_root_command()
{
    last_command="dsops"

    command_aliases=()

    commands=()
    commands+=("completion")
    commands+=("doctor")
    commands+=("exec")
    commands+=("get")
    commands+=("guard")
    commands+=("help")
    commands+=("init")
    commands+=("install-hook")
    commands+=("leak")
    commands+=("login")
    commands+=("plan")
    commands+=("providers")
    commands+=("render")
    commands+=("rotation")
    commands+=("secrets")
    commands+=("shred")

    flags=()
    two_word_flags=()
    local_nonpersistent_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--config=")
    two_word_flags+=("--config")
    flags+=("--debug")
    flags+=("--no-color")
    flags+=("--non-interactive")

    must_have_one_flag=()
    must_have_one_noun=()
    noun_aliases=()
}

__start_dsops()
{
    local cur prev words cword split
    declare -A flaghash 2>/dev/null || :
    declare -A aliashash 2>/dev/null || :
    if declare -F _init_completion >/dev/null 2>&1; then
        _init_completion -s || return
    else
        __dsops_init_completion -n "=" || return
    fi

    local c=0
    local flag_parsing_disabled=
    local flags=()
    local two_word_flags=()
    local local_nonpersistent_flags=()
    local flags_with_completion=()
    local flags_completion=()
    local commands=("dsops")
    local command_aliases=()
    local must_have_one_flag=()
    local must_have_one_noun=()
    local has_completion_function=""
    local last_command=""
    local nouns=()
    local noun_aliases=()

    __dsops_handle_word
}

if [[ $(type -t compopt) = "builtin" ]]; then
    complete -o default -F __start_dsops dsops
else
    complete -o default -o nospace -F __start_dsops dsops
fi

# ex: ts=4 sw=4 et filetype=sh
