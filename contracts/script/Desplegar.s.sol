// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import {Script} from "forge-std/Script.sol";
import {console2} from "forge-std/console2.sol";

import {Distinciones} from "../src/Distinciones.sol";

/// @title Desplegar — el contrato con su catálogo, una sola vez.
///
/// @notice Todo lo que cambia entre un despliegue y otro entra por variables de
///         entorno. Acá adentro no hay ninguna clave, ninguna dirección y
///         ninguna red: si hubiera, este archivo no podría estar en el repo.
///
/// @dev Se corre así (las variables, en `contracts/.env`, que está gitignoreado):
///
///      ```sh
///      forge script script/Desplegar.s.sol:Desplegar \
///          --rpc-url "$RPC_URL" --broadcast --verify -vvvv
///      ```
///
///      El detalle de cada variable, y cómo se consigue plata de testnet, está
///      en `README.md`.
contract Desplegar is Script {
    /// @notice El catálogo no tiene las 41 distinciones que el diseño promete.
    /// @dev No es un capricho: el catálogo entra una sola vez y no se puede
    ///      corregir. Desplegar con veinte piezas es quedarse sin las otras
    ///      veintiuna para siempre.
    error CatalogoIncompleto(uint256 cuantas);
    /// @notice El JSON no tiene ninguna distinción adentro.
    error CatalogoVacio(string ruta);
    /// @notice Un número del catálogo no entra en el `uint8` del struct.
    error CampoFueraDeRango(uint256 indice, uint256 valor);

    /// @dev Cuántas distinciones tiene que haber: 34 piezas y 7 obras.
    uint256 internal constant CUANTAS = 41;

    function run() external returns (Distinciones desplegado) {
        address duenio = vm.envAddress("DUENIO");
        address firmante = vm.envAddress("FIRMANTE");
        string memory ruta = vm.envOr("CATALOGO", string("script/catalogo.json"));
        // En Base Sepolia no importa; el día que se toque una red con plata de
        // verdad, esto obliga a mirar el catálogo antes de firmar.
        bool permitirIncompleto = vm.envOr("CATALOGO_INCOMPLETO", false);

        Distinciones.Pieza[] memory catalogo = leerCatalogo(ruta);
        if (!permitirIncompleto && catalogo.length != CUANTAS) {
            revert CatalogoIncompleto(catalogo.length);
        }

        console2.log(unicode"catálogo:", ruta);
        console2.log("distinciones:", catalogo.length);
        console2.log(unicode"dueño:", duenio);
        console2.log("firmante:", firmante);
        console2.log("chainId:", block.chainid);

        vm.startBroadcast();
        desplegado = new Distinciones(duenio, firmante, catalogo);
        vm.stopBroadcast();

        console2.log("Distinciones:", address(desplegado));
        console2.log(
            unicode"anotá esta dirección: el servidor la necesita para firmar"
            unicode" (entra en el dominio EIP-712)"
        );
    }

    /// @notice Lee el catálogo del JSON, entrada por entrada y campo por campo.
    ///
    /// @dev Dos atajos descartados, y por qué.
    ///
    ///      `abi.decode(vm.parseJson(json), (Pieza[]))` ordena los campos
    ///      alfabéticamente y no por el orden del struct: el día que alguien
    ///      renombre un campo, el catálogo entra corrido y nadie se entera hasta
    ///      que el `tokenURI` dibuja cualquier cosa.
    ///
    ///      `.distinciones[*].nombre` sería más corto, pero el jsonpath de
    ///      Foundry devuelve un valor por consulta y rechaza el comodín. Así que
    ///      se recorre el arreglo a mano, preguntando por cada índice hasta que
    ///      no hay más. Es lento y no importa: corre una vez, fuera de la cadena.
    function leerCatalogo(string memory ruta)
        public
        view
        returns (Distinciones.Pieza[] memory catalogo)
    {
        string memory json = vm.readFile(ruta);

        uint256 n = 0;
        while (vm.keyExistsJson(json, _campo(n, "nombre"))) {
            ++n;
        }
        if (n == 0) revert CatalogoVacio(ruta);

        catalogo = new Distinciones.Pieza[](n);
        for (uint256 i; i < n; ++i) {
            uint256 obra = vm.parseJsonUint(json, _campo(i, "obra"));
            uint256 arriba = vm.parseJsonUint(json, _campo(i, "juntaArriba"));
            uint256 abajo = vm.parseJsonUint(json, _campo(i, "juntaAbajo"));

            // El JSON trae `uint256`; el struct guarda `uint8`. Truncar en
            // silencio acá dejaría una obra 300 convertida en obra 44.
            if (obra > type(uint8).max) revert CampoFueraDeRango(i, obra);
            if (arriba > type(uint8).max) revert CampoFueraDeRango(i, arriba);
            if (abajo > type(uint8).max) revert CampoFueraDeRango(i, abajo);

            catalogo[i] = Distinciones.Pieza({
                nombre: vm.parseJsonString(json, _campo(i, "nombre")),
                obra: uint8(obra),
                juntaArriba: uint8(arriba),
                juntaAbajo: uint8(abajo),
                esObra: vm.parseJsonBool(json, _campo(i, "esObra"))
            });
        }
    }

    function _campo(uint256 i, string memory nombre) private pure returns (string memory) {
        return string.concat(".distinciones[", vm.toString(i), "].", nombre);
    }
}
